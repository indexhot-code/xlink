package terminal

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func testSub(t *testing.T, hardLimit int64) *subscriber {
	t.Helper()
	old := hardLimitBytes
	if hardLimit > 0 {
		hardLimitBytes = hardLimit
	}
	t.Cleanup(func() { hardLimitBytes = old })
	return newSubscriber()
}

// 队列满时 offer 阻塞（背压而非丢弃）；消费一帧后恢复流动。
func TestOfferBlocksWhenQueueFull(t *testing.T) {
	s := testSub(t, 0)

	for i := 0; i < queueLen; i++ {
		s.Offer([]byte("x"))
	}
	if s.Dropped.Load() != 0 {
		t.Fatal("no drop expected before hard limit")
	}

	blocked := make(chan struct{})
	go func() {
		s.Offer([]byte("overflow"))
		close(blocked)
	}()

	select {
	case <-blocked:
		t.Fatal("offer should block when queue is full")
	case <-time.After(100 * time.Millisecond):
		// 仍阻塞：符合预期
	}

	data := <-s.ch // 消费一帧
	s.Consume(data)

	select {
	case <-blocked:
		// 恢复流动：符合预期
	case <-time.After(time.Second):
		t.Fatal("offer should unblock after consume")
	}
}

// 超过硬上限 → 丢弃并计数，不阻塞、不崩溃。
func TestDropCounterOnHardLimit(t *testing.T) {
	s := testSub(t, 3) // 硬上限 3 字节，便于测试

	s.Offer([]byte("ab"))   // pending=2，入队
	s.Offer([]byte("cdef")) // 2+4 > 3 → 丢弃
	if s.Dropped.Load() != 1 {
		t.Fatalf("want 1 drop, got %d", s.Dropped.Load())
	}
	// 队列里只有第一帧
	if got := <-s.ch; string(got) != "ab" {
		t.Fatalf("unexpected frame %q", got)
	}
}

// 并发压测：多生产者多消费者，丢弃只可能发生在硬上限处，绝不 panic。
func TestConcurrentOfferConsume(t *testing.T) {
	s := testSub(t, 0)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			payload := bytes.Repeat([]byte(fmt.Sprintf("%d", n)), 16)
			for {
				select {
				case <-stop:
					return
				default:
					s.Offer(payload)
				}
			}
		}(i)
	}
	consumed := 0
	deadline := time.After(300 * time.Millisecond)
loop:
	for {
		select {
		case data := <-s.ch:
			s.Consume(data)
			consumed++
		case <-deadline:
			break loop
		}
	}
	close(stop)
	// 生产者可能仍阻塞在 Offer 中：持续排水直到全部退出。
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()
drain:
	for {
		select {
		case data := <-s.ch:
			s.Consume(data)
			consumed++
		case <-wgDone:
			break drain
		}
	}
	if consumed == 0 {
		t.Fatal("consumer should have consumed frames")
	}
}

// next 语义：ctx 取消后返回 false。
func TestConsumeContextCancel(t *testing.T) {
	s := newSubscriber()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_, ok := s.next(ctx)
		if ok {
			t.Error("next should return false after cancel")
		}
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("next did not observe cancel")
	}
}
