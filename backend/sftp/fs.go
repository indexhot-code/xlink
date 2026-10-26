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

// srcOpener 打开源端（只读，绝不修改源文件）：返回可 Seek 读流，Close 可选。
type srcOpener func(offset int64) (io.ReadSeeker, int64, int64, error)

// dstOpener 打开目标端：offset>0 定位续传，否则截断新建。
// 返回 (读写端, 文件大小, 源mtime unix nanos, 错误)。
type dstOpener func(offset int64) (io.ReadWriteSeeker, int64, int64, error)
