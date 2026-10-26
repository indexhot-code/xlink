package sftp

import (
	"context"
	"testing"
	"time"
)

func registerJob(t *testing.T, e *engine, local, remote string, size int64) *job {
	t.Helper()
	j := newTestJob(0, local, remote, size)
	id, err := e.enqueueRow(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	j.id = id
	e.mu.Lock()
	e.jobs[id] = j
	e.mu.Unlock()
	return j
}

// 分块搬运：成功后目标与源字节一致。
func TestChunkedTransferSuccess(t *testing.T) {
	fs := newMemFS()
	data := make([]byte, 100_000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	fs.put("/src", data)

	e := testEngine(t, defaultTestSettings(), fs)
	j := registerJob(t, e, "/src", "/dst", int64(len(data)))
	e.startJob(j)

	if !waitDone(j, 5*time.Second) {
		t.Fatal("transfer did not finish")
	}
	if j.state != StateSuccess {
		t.Fatalf("state=%s err=%s", j.state, j.err)
	}
	got := fs.get("/dst")
	if len(got) != len(data) {
		t.Fatalf("size mismatch: %d vs %d", len(got), len(data))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}

// 暂停停在 chunk 边界；续传从偏移继续，最终内容一致。
func TestPauseResumeContinuesFromOffset(t *testing.T) {
	fs := newMemFS()
	data := make([]byte, 2_000_000)
	for i := range data {
		data[i] = byte(i * 7 % 253)
	}
	fs.put("/src", data)

	e := testEngine(t, defaultTestSettings(), fs) // 8KB chunk
	j := registerJob(t, e, "/src", "/dst", int64(len(data)))
	e.startJob(j)
	waitForOffset(t, j, 1, int64(len(data))-1)
	if err := e.Pause(j.id); err != nil {
		t.Fatal(err)
	}
	if j.state != StatePaused {
		t.Fatalf("want paused, got %s (%s)", j.state, j.err)
	}
	if j.offset == 0 || j.offset >= int64(len(data)) {
		t.Fatalf("suspicious offset %d", j.offset)
	}
	if j.offset%int64(8*1024) != 0 {
		t.Fatalf("offset %d not chunk-aligned", j.offset)
	}

	if err := e.Resume(context.Background(), j.id); err != nil {
		t.Fatal(err)
	}
	if !waitDone(j, 10*time.Second) {
		t.Fatal("resume did not finish")
	}
	if j.state != StateSuccess {
		t.Fatalf("state=%s err=%s", j.state, j.err)
	}
	got := fs.get("/dst")
	if len(got) != len(data) {
		t.Fatalf("size mismatch: %d vs %d", len(got), len(data))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}

// 取消后 Retry：从保留的偏移继续到成功。
func TestCancelThenRetry(t *testing.T) {
	fs := newMemFS()
	data := make([]byte, 2_000_000)
	fs.put("/src", data)

	e := testEngine(t, defaultTestSettings(), fs)
	j := registerJob(t, e, "/src", "/dst", int64(len(data)))
	e.startJob(j)
	waitForOffset(t, j, 1, int64(len(data))-1)

	if err := e.Cancel(j.id); err != nil {
		t.Fatal(err)
	}
	if j.state != StateCanceled {
		t.Fatalf("want canceled, got %s", j.state)
	}
	if j.offset == 0 {
		t.Fatal("cancel should preserve offset for retry")
	}
	off := j.offset

	if err := e.Resume(context.Background(), j.id); err != nil {
		t.Fatal(err)
	}
	if !waitDone(j, 10*time.Second) {
		t.Fatal("retry did not finish")
	}
	if j.state != StateSuccess {
		t.Fatalf("state=%s err=%s", j.state, j.err)
	}
	if j.offset < off {
		t.Fatal("offset went backwards after retry")
	}
	if got := fs.get("/dst"); len(got) != len(data) {
		t.Fatalf("size mismatch: %d", len(got))
	}
}

// 源文件大小变化 → error。
func TestSourceSizeChanged(t *testing.T) {
	fs := newMemFS()
	fs.put("/src", []byte("hello"))

	e := testEngine(t, defaultTestSettings(), fs)
	j := registerJob(t, e, "/src", "/dst", 999)
	e.startJob(j)
	waitDone(j, 5*time.Second)

	if j.state != StateError {
		t.Fatalf("want error, got %s (%s)", j.state, j.err)
	}
}
