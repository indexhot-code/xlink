package sftp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"xlink/backend/store"
)

// memFile 是内存 ReadWriteSeeker（fake 两端）。
type memFile struct {
	data []byte
	pos  int64
}

func (m *memFile) Read(p []byte) (int, error) {
	if m.pos >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.pos:])
	m.pos += int64(n)
	return n, nil
}

func (m *memFile) Write(p []byte) (int, error) {
	end := m.pos + int64(len(p))
	if end > int64(len(m.data)) {
		m.data = append(m.data, make([]byte, end-int64(len(m.data)))...)
	}
	copy(m.data[m.pos:], p)
	m.pos = end
	return len(p), nil
}

func (m *memFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = offset
	case io.SeekCurrent:
		m.pos += offset
	case io.SeekEnd:
		m.pos = int64(len(m.data)) + offset
	}
	return m.pos, nil
}

// memFS 持有多个内存文件。
type memFS struct {
	mu    sync.Mutex
	files map[string]*memFile
}

func newMemFS() *memFS { return &memFS{files: make(map[string]*memFile)} }

func (m *memFS) put(path string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = &memFile{data: append([]byte(nil), data...)}
}

func (m *memFS) get(path string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.files[path]
	if f == nil {
		return nil
	}
	return append([]byte(nil), f.data...)
}

func (m *memFS) opener(path string) sideOpener {
	return func(offset int64) (io.ReadWriteSeeker, int64, int64, error) {
		m.mu.Lock()
		f, ok := m.files[path]
		if !ok {
			if offset > 0 {
				m.mu.Unlock()
				return nil, 0, 0, os.ErrNotExist
			}
			f = &memFile{}
			m.files[path] = f
		}
		if offset > 0 && int64(len(f.data)) < offset {
			m.mu.Unlock()
			return nil, 0, 0, os.ErrInvalid
		}
		size := int64(len(f.data))
		m.mu.Unlock()
		if offset > 0 {
			f.Seek(offset, io.SeekStart)
		}
		return f, size, 0, nil
	}
}

func testEngine(t *testing.T, set Settings, fs *memFS) *engine {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	db, err := store.Open(store.DefaultDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return newEngine(db, set, func(j *job) (sideOpener, sideOpener) {
		return fs.opener(j.localPath), fs.opener(j.remotePath)
	})
}

func newTestJob(id int64, local, remote string, size int64) *job {
	return &job{
		id: id, dir: Upload, sessionID: "s1", hostID: 1,
		localPath: local, remotePath: remote, size: size,
		pauseCh: make(chan struct{}), cancelCh: make(chan struct{}), doneCh: make(chan struct{}),
	}
}

func waitDone(j *job, d time.Duration) bool {
	select {
	case <-j.doneCh:
		return true
	case <-time.After(d):
		return false
	}
}

func defaultTestSettings() Settings {
	return Settings{ChunkSizeKB: 8, ResumeMode: "size_mtime", Overwrite: "overwrite"}
}
// waitForOffset 轮询等待任务偏移进入 [min,max) 区间（确定性暂停时机）。
func waitForOffset(t *testing.T, j *job, min, max int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if j.offset >= min && j.offset < max {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("offset %d never reached [%d,%d)", j.offset, min, max)
}
