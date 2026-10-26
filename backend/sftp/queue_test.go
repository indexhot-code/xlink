package sftp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xlink/backend/store"
)

// 队列持久化：入队 → 新引擎 loadQueue → 偏移保留，running 恢复为 paused。
func TestQueuePersistence(t *testing.T) {
	fs := newMemFS()
	path := filepath.Join(t.TempDir(), "q.db")
	db, err := store.Open(store.DefaultDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	openSide := func(j *job) (srcOpener, dstOpener) {
		return wrapSrc(fs.opener(j.localPath)), fs.opener(j.remotePath)
	}
	e1 := newEngine(db, defaultTestSettings(), openSide)
	j := newTestJob(0, "/src", "/dst", 10)
	id, err := e1.enqueueRow(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE sftp_queue SET offset=4096, state='running' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}

	// 新引擎（模拟进程重启）
	e2 := newEngine(db, defaultTestSettings(), openSide)
	if err := e2.loadQueue(); err != nil {
		t.Fatal(err)
	}
	got := e2.list()
	if len(got) != 1 {
		t.Fatalf("want 1 job, got %d", len(got))
	}
	tr := got[0]
	if tr.ID != id || tr.Transferred != 4096 {
		t.Fatalf("unexpected transfer: %+v", tr)
	}
	if tr.State != StatePaused {
		t.Fatalf("running should restore as paused, got %s", tr.State)
	}
}

// sha256 模式：前缀被篡改 → overwrite 策略下清零重传成功。
func TestSha256PrefixMismatchRestarts(t *testing.T) {
	fs := newMemFS()
	data := make([]byte, 3_000_000)
	for i := range data {
		data[i] = byte(i % 249)
	}
	fs.put("/src", data)

	set := Settings{ChunkSizeKB: 8, ResumeMode: "sha256", Overwrite: "overwrite"}
	e := testEngine(t, set, fs)
	j := registerJob(t, e, "/src", "/dst", int64(len(data)))
	e.startJob(j)
	waitForOffset(t, j, 1<<20, int64(len(data))-1) // >1MB 触发周期持久化
	if err := e.Pause(j.id); err != nil {
		t.Fatal(err)
	}
	if j.prefixHash == "" {
		t.Fatal("sha256 mode should record prefix hash on persist")
	}

	// 篡改目标已传输前缀，模拟部分数据损坏
	mf := fs.files["/dst"]
	if mf == nil || len(mf.data) == 0 {
		t.Fatal("partial dst missing")
	}
	mf.data[0] ^= 0xFF

	if err := e.Resume(context.Background(), j.id); err != nil {
		t.Fatal(err)
	}
	if !waitDone(j, 10*time.Second) {
		t.Fatal("restart did not finish")
	}
	if j.state != StateSuccess {
		t.Fatalf("state=%s err=%s", j.state, j.err)
	}
	if j.offset != int64(len(data)) {
		t.Fatalf("final offset %d, want %d", j.offset, len(data))
	}
	got := fs.get("/dst")
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("byte mismatch at %d after restart", i)
		}
	}
}
