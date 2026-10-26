package terminal

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// wsServer 是进程内回环 WebSocket 服务器（架构规则 1）：
//   - 仅绑定 127.0.0.1 随机端口；
//   - 32 字节随机令牌握手校验，不落盘、不进前端 localStorage；
//   - 校验失败返回 403；
//   - 二进制帧 = PTY 原始字节；文本帧 = JSON 控制消息（resize/exit/state）。
type wsServer struct {
	ln       net.Listener
	http     *http.Server
	token    []byte // 32 字节，仅内存
	port     int
	mu       sync.Mutex
	sessions map[string]*session
}

func newWSServer() (*wsServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen loopback: %w", err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		ln.Close()
		return nil, fmt.Errorf("generate token: %w", err)
	}
	w := &wsServer{
		ln:       ln,
		token:    token,
		port:     ln.Addr().(*net.TCPAddr).Port,
		sessions: make(map[string]*session),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pty/{id}", w.handlePTY)
	w.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := w.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[terminal] ws server: %v", err)
		}
	}()
	return w, nil
}

func (w *wsServer) put(id string, s *session) {
	w.mu.Lock()
	w.sessions[id] = s
	w.mu.Unlock()
}

func (w *wsServer) remove(id string) {
	w.mu.Lock()
	delete(w.sessions, id)
	w.mu.Unlock()
}

func (w *wsServer) get(id string) *session {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sessions[id]
}

func (w *wsServer) closeAll() {
	w.mu.Lock()
	ss := make([]*session, 0, len(w.sessions))
	for _, s := range w.sessions {
		ss = append(ss, s)
	}
	w.sessions = make(map[string]*session)
	w.mu.Unlock()
	for _, s := range ss {
		s.Close()
	}
	_ = w.http.Close()
}

func (w *wsServer) tokenHex() string { return hex.EncodeToString(w.token) }

// handlePTY 处理 /pty/<sessionId>?token=<t>。
func (w *wsServer) handlePTY(resp http.ResponseWriter, req *http.Request) {
	// 握手校验：令牌常量时间比较，失败一律 403。
	qt := req.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(qt), []byte(w.tokenHex())) != 1 {
		http.Error(resp, "forbidden", http.StatusForbidden)
		return
	}
	id := req.PathValue("id")
	sess := w.get(id)
	if sess == nil {
		http.Error(resp, "not found", http.StatusNotFound)
		return
	}

	ws, err := websocket.Accept(resp, req, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1:*", "wails.localhost", "localhost:*"},
	})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 写循环：订阅者队列 → 二进制帧；会话退出 → exit 文本帧。
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for {
			select {
			case data := <-sess.sub.ch:
				sess.sub.Consume(data)
				if err := ws.Write(ctx, websocket.MessageBinary, data); err != nil {
					return
				}
			case <-sess.done:
				_ = ws.Write(ctx, websocket.MessageText, exitMessage(sess.ExitCode()))
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// 读循环：二进制帧 → PTY stdin；文本帧 → JSON 控制消息。
	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			break
		}
		if typ == websocket.MessageText {
			if cerr := applyControl(sess, data); cerr != nil {
				log.Printf("[terminal/%s] control: %v", sess.id, cerr)
			}
			continue
		}
		if _, err := sess.WriteInput(data); err != nil {
			break
		}
	}

	cancel()
	<-writeDone
	sess.Close()
	w.remove(id)
}

// controlMessage 是前端→Go 的文本帧。
type controlMessage struct {
	Type string `json:"type"`
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// applyControl 处理 resize 等控制消息。
func applyControl(sess *session, data []byte) error {
	var m controlMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("bad control frame: %w", err)
	}
	switch m.Type {
	case "resize":
		return sess.Resize(m.Cols, m.Rows)
	case "exit":
		sess.Close()
		return nil
	default:
		return nil
	}
}
