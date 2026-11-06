// Package sftp — Binding 服务层（SFTPTransferService）。
// 分块进度以原子计数维护，前端经 ListTransfers 轮询（规则 2：不走 JSON 事件）。
package sftp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"xlink/backend/store"
	"xlink/backend/terminal"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Enqueue 入队文件或目录（目录递归展开为逐文件任务）。返回创建的任务 ID。
func (s *Service) Enqueue(ctx context.Context, dir Direction, sessionID string, localPath, remotePath string) ([]int64, error) {
	switch dir {
	case Upload:
		return s.enqueueUpload(ctx, sessionID, localPath, remotePath)
	case Download:
		return s.enqueueDownload(ctx, sessionID, localPath, remotePath)
	default:
		return nil, fmt.Errorf("invalid direction %d", dir)
	}
}

func (s *Service) enqueueUpload(ctx context.Context, sessionID string, localPath, remotePath string) ([]int64, error) {
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	return s.walkLocal(ctx, sessionID, localPath, remotePath, sc)
}

// walkLocal 递归展开本地目录为逐文件任务。
func (s *Service) walkLocal(ctx context.Context, sessionID string, localPath, remotePath string, sc *sftp.Client) ([]int64, error) {
	fi, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return s.createJob(ctx, Upload, sessionID, localPath, remotePath, fi.Size(), fi.ModTime().UnixNano())
	}
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return nil, err
	}
	_ = sc.MkdirAll(remotePath) // 远端建目录（已存在则忽略）
	var ids []int64
	for _, e := range entries {
		lp := filepath.Join(localPath, e.Name())
		rp := joinRemote(remotePath, e.Name())
		sub, err := s.walkLocal(ctx, sessionID, lp, rp, sc)
		if err != nil {
			return ids, err
		}
		ids = append(ids, sub...)
	}
	return ids, nil
}

func (s *Service) enqueueDownload(ctx context.Context, sessionID string, localPath, remotePath string) ([]int64, error) {
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	return s.walkRemote(ctx, sessionID, localPath, remotePath, sc)
}

// walkRemote 递归展开远端目录为逐文件任务。
func (s *Service) walkRemote(ctx context.Context, sessionID string, localPath, remotePath string, sc *sftp.Client) ([]int64, error) {
	fi, err := sc.Stat(remotePath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return s.createJob(ctx, Download, sessionID, localPath, remotePath, fi.Size(), fi.ModTime().UnixNano())
	}
	entries, err := sc.ReadDir(remotePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(localPath, 0o755); err != nil { // 本地建目录
		return nil, err
	}
	var ids []int64
	for _, e := range entries {
		rp := joinRemote(remotePath, e.Name())
		lp := filepath.Join(localPath, e.Name())
		sub, err := s.walkRemote(ctx, sessionID, lp, rp, sc)
		if err != nil {
			return ids, err
		}
		ids = append(ids, sub...)
	}
	return ids, nil
}

// joinRemote 拼接远端路径（SFTP 恒用 / 分隔）。
func joinRemote(a, b string) string {
	return strings.TrimRight(a, "/") + "/" + b
}

// createJob 创建任务（写库 + 注册 + 立即启动）。
func (s *Service) createJob(ctx context.Context, dir Direction, sessionID, localPath, remotePath string, size, srcMTime int64) ([]int64, error) {
	j := &job{
		dir:        dir,
		sessionID:  sessionID,
		hostID:     s.term.HostIDOf(sessionID),
		localPath:  localPath,
		remotePath: remotePath,
		size:       size,
		srcMTime:   srcMTime,
		pauseCh:    make(chan struct{}),
		cancelCh:   make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	id, err := s.eng.enqueueRow(ctx, j)
	if err != nil {
		return nil, err
	}
	j.id = id
	s.eng.mu.Lock()
	s.eng.jobs[id] = j
	s.eng.mu.Unlock()
	s.eng.setState(j, StateRunning, "")
	s.eng.startJob(j)
	return []int64{id}, nil
}

// ---- 连接与两端打开 ----

// clientFor 返回会话的 SSH 客户端（复用终端连接）。
func (s *Service) clientFor(sessionID string) (*ssh.Client, error) {
	c := s.term.GetClient(sessionID)
	if c == nil {
		return nil, fmt.Errorf("session %s not connected", sessionID)
	}
	return c, nil
}

// sftpClient 基于会话的 SSH 连接派生一次性 sftp 子连接。
func (s *Service) sftpClient(sessionID string) (*sftp.Client, func(), error) {
	client, err := s.clientFor(sessionID)
	if err != nil {
		return nil, nil, err
	}
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, nil, fmt.Errorf("sftp sub-client: %w", err)
	}
	return sc, func() { sc.Close() }, nil
}

// openSide 返回任务两端打开器（上传：local→remote；下载：remote→local）。
// 源端只读（绝不截断/修改源文件）；目标端 offset==0 时截断新建。
func (s *Service) openSide(j *job) (src srcOpener, dst dstOpener) {
	if j.dir == Upload {
		return s.openLocalRead(j), s.openRemoteWrite(j)
	}
	return s.openRemoteRead(j), s.openLocalWrite(j)
}

// openLocalRead 源端只读。
func (s *Service) openLocalRead(j *job) srcOpener {
	return func(offset int64) (io.ReadSeeker, int64, int64, error) {
		f, err := os.Open(j.localPath)
		if err != nil {
			return nil, 0, 0, err
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, 0, err
		}
		if offset > 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				return nil, 0, 0, err
			}
		}
		return f, fi.Size(), fi.ModTime().UnixNano(), nil
	}
}

// openLocalWrite 目标端：offset>0 定位续传，否则截断新建。
func (s *Service) openLocalWrite(j *job) dstOpener {
	return func(offset int64) (io.ReadWriteSeeker, int64, int64, error) {
		flags := os.O_RDWR | os.O_CREATE
		if offset == 0 {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(j.localPath, flags, 0o644)
		if err != nil {
			return nil, 0, 0, err
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, 0, err
		}
		if offset > 0 {
			if fi.Size() < offset {
				f.Close()
				return nil, 0, 0, fmt.Errorf("local target shorter than offset")
			}
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				return nil, 0, 0, err
			}
		}
		return f, fi.Size(), fi.ModTime().UnixNano(), nil
	}
}

// openRemoteRead 源端只读（文件关闭时一并关闭子连接）。
func (s *Service) openRemoteRead(j *job) srcOpener {
	return func(offset int64) (io.ReadSeeker, int64, int64, error) {
		sc, closeFn, err := s.sftpClient(j.sessionID)
		if err != nil {
			return nil, 0, 0, err
		}
		f, err := sc.Open(j.remotePath)
		if err != nil {
			closeFn()
			return nil, 0, 0, err
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			closeFn()
			return nil, 0, 0, err
		}
		if offset > 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				closeFn()
				return nil, 0, 0, err
			}
		}
		return &remoteFile{File: f, closeFn: closeFn}, fi.Size(), fi.ModTime().UnixNano(), nil
	}
}

// openRemoteWrite 目标端：同 openLocalWrite 语义。
func (s *Service) openRemoteWrite(j *job) dstOpener {
	return func(offset int64) (io.ReadWriteSeeker, int64, int64, error) {
		sc, closeFn, err := s.sftpClient(j.sessionID)
		if err != nil {
			return nil, 0, 0, err
		}
		flags := os.O_RDWR | os.O_CREATE
		if offset == 0 {
			flags |= os.O_TRUNC
		}
		f, err := sc.OpenFile(j.remotePath, flags)
		if err != nil {
			closeFn()
			return nil, 0, 0, err
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			closeFn()
			return nil, 0, 0, err
		}
		if offset > 0 {
			if fi.Size() < offset {
				f.Close()
				closeFn()
				return nil, 0, 0, fmt.Errorf("remote target shorter than offset")
			}
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				closeFn()
				return nil, 0, 0, err
			}
		}
		return &remoteFile{File: f, closeFn: closeFn}, fi.Size(), fi.ModTime().UnixNano(), nil
	}
}

// remoteFile 包装 sftp.File：Close 时同时释放一次性子连接。
type remoteFile struct {
	*sftp.File
	closeFn func()
}

func (r *remoteFile) Close() error {
	err := r.File.Close()
	r.closeFn()
	return err
}

// ---- 目录列表 Binding ----

// RemoteEntry / LocalEntry 目录条目。
type RemoteEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

type LocalEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// ListDir 列出远端目录（目录在前、按名称排序）。
func (s *Service) ListDir(ctx context.Context, sessionID, path string) ([]RemoteEntry, error) {
	sc, closeFn, err := s.sftpClient(sessionID)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	entries, err := sc.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	out := make([]RemoteEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, RemoteEntry{
			Name: e.Name(), IsDir: e.IsDir(), Size: e.Size(), ModTime: e.ModTime().UnixNano(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ListLocalDir 列出本地目录（裸盘符 "D:" 规范化为 "D:\"；目录在前、按名称排序）。
func (s *Service) ListLocalDir(ctx context.Context, path string) ([]LocalEntry, error) {
	if len(path) == 2 && path[1] == ':' {
		path += `\`
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	out := make([]LocalEntry, 0, len(entries))
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out = append(out, LocalEntry{
			Name: e.Name(), IsDir: e.IsDir(), Size: info.Size(), ModTime: info.ModTime().UnixNano(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ---- 传输列表 Binding ----

// Transfer 是 ListTransfers 返回的任务快照。
type Transfer struct {
	ID          int64         `json:"id"`
	Direction   Direction     `json:"direction"`
	SessionID   string        `json:"sessionId"`
	HostID      int64         `json:"hostId"`
	LocalPath   string        `json:"localPath"`
	RemotePath  string        `json:"remotePath"`
	Size        int64         `json:"size"`
	Transferred int64         `json:"transferred"`
	State       TransferState `json:"state"`
	Error       string        `json:"error"`
}

func (s *Service) ListTransfers(ctx context.Context) ([]Transfer, error) { return s.eng.list(), nil }

// ---- 设置 ----

// Settings 是全局传输设置（存 settings 表，不在 UI 硬编码）。
type Settings struct {
	ChunkSizeKB int    `json:"chunkSizeKB"` // 默认 512
	ResumeMode  string `json:"resumeMode"`  // size_mtime(默认) | sha256 | off
	Overwrite   string `json:"overwrite"`   // overwrite(默认) | skip
}

func defaultSettings() Settings {
	return Settings{ChunkSizeKB: 512, ResumeMode: "size_mtime", Overwrite: "overwrite"}
}

// Service 是 Wails 绑定的 SFTP 传输服务。
type Service struct {
	term *terminal.Service
	db   *store.Store
	lastDirs LastDirs

	set Settings
	eng *engine
}

func New(term *terminal.Service, db *store.Store) *Service {
	return &Service{term: term, db: db, set: defaultSettings()}
}

func (s *Service) ServiceName() string { return "SFTPTransferService" }

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := s.loadSettings(); err != nil {
		return fmt.Errorf("load sftp settings: %w", err)
	}
	// 加载上次所在目录
	var rawld string
	if err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, lastDirsKey).Scan(&rawld); err == nil {
		_ = json.Unmarshal([]byte(rawld), &s.lastDirs)
	}
	s.eng = newEngine(s.db, s.set, s.openSide)
	if err := s.eng.loadQueue(); err != nil {
		return fmt.Errorf("restore sftp queue: %w", err)
	}
	return nil
}

func (s *Service) ServiceShutdown() error {
	// 运行中任务取消（偏移已持久化，重启后可续传）。
	for _, t := range s.eng.list() {
		if t.State == StateRunning || t.State == StatePending {
			_ = s.eng.Cancel(t.ID)
		}
	}
	return nil
}

func (s *Service) loadSettings() error {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='sftp'`).Scan(&raw)
	if err != nil {
		return nil // 无记录 → 默认值
	}
	return json.Unmarshal([]byte(raw), &s.set)
}

func (s *Service) GetSettings(ctx context.Context) (Settings, error) { return s.set, nil }

func (s *Service) SaveSettings(ctx context.Context, in Settings) error {
	if in.ChunkSizeKB < 4 || in.ChunkSizeKB > 4096 {
		return fmt.Errorf("chunkSizeKB must be in [4,4096]")
	}
	switch in.ResumeMode {
	case "size_mtime", "sha256", "off":
	default:
		return fmt.Errorf("invalid resumeMode %q", in.ResumeMode)
	}
	switch in.Overwrite {
	case "overwrite", "skip":
	default:
		return fmt.Errorf("invalid overwrite %q", in.Overwrite)
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings(key, value) VALUES('sftp', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(raw)); err != nil {
		return err
	}
	s.set = in
	if s.eng != nil {
		s.eng.settings = in
	}
	return nil
}

// ---- 队列操作（Binding）----

func (s *Service) Pause(ctx context.Context, id int64) error { return s.eng.Pause(id) }

// resumeChecked 恢复前校验任务所属会话仍连接，避免重启 worker 后立即再报错（前端表现为无响应）。
func (s *Service) resumeChecked(ctx context.Context, id int64) error {
	if j := s.eng.jobLocked(id); j != nil && s.term.GetClient(j.sessionID) == nil {
		return fmt.Errorf("session %s not connected, reconnect first", j.sessionID)
	}
	return s.eng.Resume(ctx, id)
}

func (s *Service) Resume(ctx context.Context, id int64) error { return s.resumeChecked(ctx, id) }
func (s *Service) Cancel(ctx context.Context, id int64) error { return s.eng.Cancel(id) }
func (s *Service) Retry(ctx context.Context, id int64) error  { return s.resumeChecked(ctx, id) }

// OnSessionClosed 标记某会话的运行中任务为 error（连接断开时由终端侧调用）。
func (s *Service) OnSessionClosed(sessionID string) {
	if s.eng != nil {
		s.eng.OnSessionClosed(sessionID)
	}
}
