package sftp

import (
	"context"
	"fmt"
	"io"
)

const persistEveryBytes = 1 << 20 // 1MB：偏移持久化间隔

// runTransfer 是任务 worker：分块搬运 + 暂停/取消 + 续传。
// 假定状态已是 running（由调用方设置）。
func (e *engine) runTransfer(j *job) {
	defer close(j.doneCh)

	srcOpen, dstOpen := e.openSide(j)
	src, srcSize, srcMTime, err := srcOpen(0)
	if err != nil {
		j.offset = 0
		j.transferred.Store(0)
		e.setState(j, StateError, "open source: "+err.Error())
		return
	}
	if c, ok := src.(io.Closer); ok {
		defer c.Close()
	}
	dst, _, _, err := dstOpen(j.offset)
	if err != nil {
		e.setState(j, StateError, "open target: "+err.Error())
		return
	}
	if c, ok := dst.(io.Closer); ok {
		defer c.Close()
	}

	// 源一致性校验（ResumeMode=off 时跳过）。
	if e.settings.ResumeMode != "off" {
		if srcSize > 0 && j.size > 0 && srcSize != j.size {
			e.setState(j, StateError, "source size changed")
			return
		}
		if j.srcMTime > 0 && srcMTime > 0 && srcMTime != j.srcMTime {
			e.setState(j, StateError, "source mtime changed")
			return
		}
		if j.prefixHash != "" {
			// 校验目标端已传输前缀（部分数据在目标侧）
			h, herr := hashPrefix(dst, j.offset)
			if herr != nil || h != j.prefixHash {
				if e.settings.Overwrite == "overwrite" {
					j.offset = 0
					j.transferred.Store(0)
					if _, serr := src.Seek(0, io.SeekStart); serr != nil {
						e.setState(j, StateError, "seek: "+serr.Error())
						return
					}
					if _, serr := dst.Seek(0, io.SeekStart); serr != nil {
						e.setState(j, StateError, "seek: "+serr.Error())
						return
					}
					j.prefixHash = ""
				} else {
					e.setState(j, StateError, "resume prefix hash mismatch")
					return
				}
			}
		}
	}

	buf := make([]byte, e.settings.ChunkSizeKB*1024)
	lastPersist := j.offset
	for {
		if p, c := e.stopRequested(j); p || c {
			if p {
				// 暂停：落盘偏移（sha256 模式同时记录目标端前缀哈希）
				e.persistOffset(j, func() (string, error) { return hashPrefix(dst, j.offset) })
				e.setState(j, StatePaused, "")
			} else {
				e.setState(j, StateCanceled, "")
			}
			return
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				e.setState(j, StateError, "write: "+werr.Error())
				return
			}
			j.offset += int64(n)
			j.transferred.Store(j.offset)
			if j.offset-lastPersist >= persistEveryBytes {
				e.persistOffset(j, func() (string, error) { return hashPrefix(src, j.offset) })
				lastPersist = j.offset
			}
		}
		if rerr == io.EOF {
			e.persistOffset(j, nil)
			e.setState(j, StateSuccess, "")
			return
		}
		if rerr != nil {
			e.setState(j, StateError, "read: "+rerr.Error())
			return
		}
	}
}

// startJob 启动 worker（状态置 running）。
func (e *engine) startJob(j *job) {
	go e.runTransfer(j)
}

// Pause 请求暂停（chunk 边界生效），阻塞到 worker 退出。
func (e *engine) Pause(id int64) error {
	j := e.jobLocked(id)
	if j == nil {
		return fmt.Errorf("job %d not found", id)
	}
	select {
	case <-j.pauseCh:
	default:
		close(j.pauseCh)
	}
	<-j.doneCh
	return nil
}

// Cancel 取消任务（保留偏移，可 Retry 续传）。
func (e *engine) Cancel(id int64) error {
	j := e.jobLocked(id)
	if j == nil {
		return fmt.Errorf("job %d not found", id)
	}
	select {
	case <-j.cancelCh:
	default:
		close(j.cancelCh)
	}
	<-j.doneCh
	return nil
}

// Resume 恢复已停止（paused/canceled/error/success 重跑）的任务。
func (e *engine) Resume(ctx context.Context, id int64) error {
	j := e.jobLocked(id)
	if j == nil {
		return fmt.Errorf("job %d not found", id)
	}
	select {
	case <-j.doneCh:
	default:
		return fmt.Errorf("job %d still running", id)
	}
	// 重建生命周期通道
	j.pauseCh = make(chan struct{})
	j.cancelCh = make(chan struct{})
	j.doneCh = make(chan struct{})
	e.startJob(j)
	return nil
}

// jobLocked 取任务（不复制）。
func (e *engine) jobLocked(id int64) *job {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.jobs[id]
}

// loadQueue 从 DB 恢复任务（启动时调用）。
// 杀进程重启：pending/running 一律恢复为 paused，偏移已持久化，Resume 后从偏移继续。
func (e *engine) loadQueue() error {
	rows, err := e.db.Query(
		`SELECT id, direction, session_id, host_id, local_path, remote_path, size, offset, src_mtime, prefix_hash, state, error
		 FROM sftp_queue ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	// 先完整读完结果集（单连接池：rows 未关闭时不可再 Exec），
	// 需要改状态的任务收集到最后统一处理。
	type restoreState struct {
		j      *job
		toPause bool
		errMsg string
	}
	var pending []restoreState
	for rows.Next() {
		var j job
		var dir int
		var state, errMsg, phash, sessionID string
		if err := rows.Scan(&j.id, &dir, &sessionID, &j.hostID, &j.localPath, &j.remotePath,
			&j.size, &j.offset, &j.srcMTime, &phash, &state, &errMsg); err != nil {
			return err
		}
		j.dir = Direction(dir)
		j.sessionID = sessionID
		j.prefixHash = phash
		j.transferred.Store(j.offset)
		// 初始化生命周期通道并立即 close doneCh：表示未在运行，Resume 可重新 startJob。
		j.pauseCh = make(chan struct{})
		j.cancelCh = make(chan struct{})
		j.doneCh = make(chan struct{})
		close(j.doneCh)
		jp := &j
		e.mu.Lock()
		e.jobs[j.id] = jp
		e.mu.Unlock()
		st := TransferState(state)
		if st == StatePending || st == StateRunning {
			pending = append(pending, restoreState{j: jp, toPause: true, errMsg: errMsg})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range pending {
		e.setState(p.j, StatePaused, p.errMsg)
	}
	return nil
}

// OnSessionClosed 标记某会话的运行中任务为 error（连接断开）。
func (e *engine) OnSessionClosed(sessionID string) {
	e.mu.Lock()
	js := make([]*job, 0, 4)
	for _, j := range e.jobs {
		if j.sessionID == sessionID {
			js = append(js, j)
		}
	}
	e.mu.Unlock()
	for _, j := range js {
		select {
		case <-j.doneCh:
			continue
		default:
			close(j.cancelCh)
			<-j.doneCh
			e.setState(j, StateError, "session closed")
		}
	}
}

// list 返回全部任务快照（状态以内存为准，由 setState 维护）。
func (e *engine) list() []Transfer {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Transfer, 0, len(e.jobs))
	for _, j := range e.jobs {
		out = append(out, Transfer{
			ID: j.id, Direction: j.dir, SessionID: j.sessionID, HostID: j.hostID,
			LocalPath: j.localPath, RemotePath: j.remotePath,
			Size: j.size, Transferred: j.transferred.Load(),
			State: j.state, Error: j.err,
		})
	}
	return out
}
