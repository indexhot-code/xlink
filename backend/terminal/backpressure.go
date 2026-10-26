package terminal

import (
	"context"
	"sync/atomic"
)

const (
	// queueLen 订阅者队列长度（帧数）。队列满时 offer 阻塞 —— 背压策略为
	// "阻塞而非丢弃"（架构规则 3）：阻塞传导回 PTY 读 → SSH 通道窗口 → 远端。
	queueLen = 256
)

// hardLimitBytes 待发字节硬上限。仅极端场景（如窗口最小化导致消费停滞）
// 超过此值才丢弃新数据并计数，绝不崩溃。变量形式仅为便于测试注入小阈值。
var hardLimitBytes = int64(32 << 20) // 32 MiB

// subscriber 是单个 WS 客户端对应的 PTY 输出队列。
type subscriber struct {
	ch       chan []byte
	pending  atomic.Int64 // 队列中待发字节
	Dropped  atomic.Uint64 // 丢弃帧计数（仅硬上限触发）
}

func newSubscriber() *subscriber {
	return &subscriber{ch: make(chan []byte, queueLen)}
}

// Offer 投递一帧 PTY 数据。data 会被复制，调用方缓冲区可复用。
func (s *subscriber) Offer(data []byte) {
	n := len(data)
	if n == 0 {
		return
	}
	// 硬上限检查：在阻塞入队之前判断，极端场景丢数据保进程。
	if s.pending.Load()+int64(n) > hardLimitBytes {
		s.Dropped.Add(1)
		return
	}
	cp := make([]byte, n)
	copy(cp, data)
	s.pending.Add(int64(n))
	s.ch <- cp // 满时阻塞：背压的核心路径
}

// Consume 由消费者在取走一帧后调用，归减待发计数。
func (s *subscriber) Consume(data []byte) {
	s.pending.Add(-int64(len(data)))
}

// next 从队列取一帧，ctx 取消时返回 false（便于测试与受控关闭）。
func (s *subscriber) next(ctx context.Context) ([]byte, bool) {
	select {
	case data := <-s.ch:
		s.Consume(data)
		return data, true
	case <-ctx.Done():
		return nil, false
	}
}
