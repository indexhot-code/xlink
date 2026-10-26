package sftp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"xlink/backend/store"
)

// job 是一个内存态传输任务。
type job struct {
	id          int64
	dir         Direction
	sessionID   string
	hostID      int64
	localPath   string
	remotePath  string
	size        int64
	offset      int64 // 已持久化偏移
	transferred atomic.Int64
	srcMTime    int64
	prefixHash  string
	state       TransferState
	err         string

	pauseCh  chan struct{} // 关闭 = 请求暂停（chunk 边界生效）
	cancelCh chan struct{}
	doneCh   chan struct{}
}

// engine 管理传输任务：worker 协程、偏移持久化、续传校验。
// openSide 由 Service 注入（live=SSH sftp；test=内存 fake）。
type engine struct {
	db       *store.Store
	mu       sync.Mutex
	jobs     map[int64]*job
	settings Settings

	// openSide 返回任务的源/目标打开器（上传：local→remote）。
	openSide func(j *job) (src srcOpener, dst dstOpener)
}

func newEngine(db *store.Store, settings Settings, openSide func(*job) (srcOpener, dstOpener)) *engine {
	return &engine{db: db, jobs: make(map[int64]*job), settings: settings, openSide: openSide}
}

// stopRequested 在 chunk 边界检查暂停/取消请求。
func (e *engine) stopRequested(j *job) (pause bool, cancel bool) {
	select {
	case <-j.pauseCh:
		return true, false
	case <-j.cancelCh:
		return false, true
	default:
		return false, false
	}
}

// setState 更新内存态并持久化（含偏移）。
func (e *engine) setState(j *job, st TransferState, errMsg string) {
	e.mu.Lock()
	j.state = st
	j.err = errMsg
	e.mu.Unlock()
	_, err := e.db.Exec(
		`UPDATE sftp_queue SET state=?, error=?, offset=?, updated_at=datetime('now') WHERE id=?`,
		string(st), errMsg, j.offset, j.id)
	if err != nil {
		fmt.Println("[sftp] persist state:", err)
	}
}

// persistOffset 周期性落盘偏移（约每 1MB）；sha256 模式同时记录前缀哈希。
func (e *engine) persistOffset(j *job, hashPrefix func() (string, error)) {
	hash := ""
	if e.settings.ResumeMode == "sha256" && hashPrefix != nil {
		if h, err := hashPrefix(); err == nil {
			hash = h
			j.prefixHash = h
		}
	}
	_, err := e.db.Exec(
		`UPDATE sftp_queue SET offset=?, prefix_hash=?, updated_at=datetime('now') WHERE id=?`,
		j.offset, hash, j.id)
	if err != nil {
		fmt.Println("[sftp] persist offset:", err)
	}
}

// hashPrefix 计算可 Seek 读端前 n 字节的 sha256，并把读端恢复到偏移 n。
func hashPrefix(src io.ReadSeeker, n int64) (string, error) {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.CopyN(h, src, n); err != nil && err != io.EOF {
		return "", err
	}
	if _, err := src.Seek(n, io.SeekStart); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// enqueueRow 写入新任务行并返回 ID。
func (e *engine) enqueueRow(ctx context.Context, j *job) (int64, error) {
	res, err := e.db.ExecContext(ctx,
		`INSERT INTO sftp_queue(direction, session_id, host_id, local_path, remote_path, size, offset, src_mtime, state)
		 VALUES(?,?,?,?,?,?,0,?,?)`,
		int(j.dir), j.sessionID, j.hostID, j.localPath, j.remotePath, j.size, j.srcMTime, string(StatePending))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
