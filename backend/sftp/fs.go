// Package sftp 实现 SFTP 传输引擎（阶段 4）：
// 队列状态机、流式分块、断点续传、偏移持久化。
package sftp

import "io"

// Direction 传输方向。
type Direction int

const (
	Upload   Direction = 1
	Download Direction = 2
)

// TransferState 队列状态机。
type TransferState string

const (
	StatePending  TransferState = "pending"
	StateRunning  TransferState = "running"
	StatePaused   TransferState = "paused"
	StateSuccess  TransferState = "success"
	StateError    TransferState = "error"
	StateCanceled TransferState = "canceled"
)

// chunkSource 是分块搬运的读端（本地文件 / sftp.File 均满足）。
type chunkSource interface {
	io.Reader
	io.Seeker
	io.Closer
}

// chunkSink 是分块搬运的写端。
type chunkSink interface {
	io.Writer
	io.Seeker
	io.Closer
}

// sideOpener 打开一端：offset>0 时定位到偏移续传，否则截断新建。
// 返回 (读写端, 文件大小, 源mtime unix nanos, 错误)。
type sideOpener func(offset int64) (io.ReadWriteSeeker, int64, int64, error)
