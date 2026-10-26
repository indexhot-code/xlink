package terminal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"golang.org/x/crypto/ssh"
)

// session 是一个已建立的 SSH PTY 会话。
type session struct {
	id      string
	client  *ssh.Client
	sshSess *ssh.Session
	stdin   io.WriteCloser
	sub     *subscriber
	done    chan struct{} // PTY 读泵全部退出后关闭
	closed  sync.Once

	exitMu  sync.Mutex
	exitCode int // 由 Wait 捕获，-1 表示未知
}

func newSession(id string, client *ssh.Client, cols, rows uint32) (*session, error) {
	sshSess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new ssh session: %w", err)
	}
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	if err := sshSess.RequestPty("xterm-256color", int(rows), int(cols), ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}); err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}
	stdin, err := sshSess.StdinPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := sshSess.StdoutPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := sshSess.StderrPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := sshSess.Shell(); err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	s := &session{
		id:      id,
		client:  client,
		sshSess: sshSess,
		stdin:   stdin,
		sub:     newSubscriber(),
		done:    make(chan struct{}),
	}

	// PTY 读泵：stdout/stderr → 订阅者队列。阻塞式 offer 提供背压。
	var pumps sync.WaitGroup
	pumps.Add(2)
	pumped := func(name string, r io.Reader) {
		defer pumps.Done()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				s.sub.Offer(buf[:n])
			}
			if rerr != nil {
				if rerr != io.EOF {
					log.Printf("[terminal/%s] %s read: %v", id, name, rerr)
				}
				return
			}
		}
	}
	go pumped("stdout", stdout)
	go pumped("stderr", stderr)
	// Wait 捕获退出码；done 在读泵与 Wait 均结束后关闭。
	go func() {
		werr := sshSess.Wait()
		pumps.Wait()
		s.exitMu.Lock()
		s.exitCode = exitStatusFromErr(werr)
		s.exitMu.Unlock()
		close(s.done)
	}()
	return s, nil
}

// Resize 调整远端 PTY 窗口。
func (s *session) Resize(cols, rows uint32) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	return s.sshSess.WindowChange(int(rows), int(cols))
}

// WriteInput 将用户输入写入 PTY stdin。
func (s *session) WriteInput(data []byte) (int, error) {
	return s.stdin.Write(data)
}

// ExitCode 返回会话退出码（仅在 done 后调用有意义）。
func (s *session) ExitCode() int {
	s.exitMu.Lock()
	defer s.exitMu.Unlock()
	return s.exitCode
}

// exitStatusFromErr 从 Session.Wait 的错误解析退出码；
// -1 表示异常关闭（网络断开、被本地关闭等）。
func exitStatusFromErr(err error) int {
	if err == nil {
		return 0
	}
	var ee *ssh.ExitError
	if errors.As(err, &ee) {
		return ee.ExitStatus()
	}
	return -1
}

// Close 关闭 SSH 会话与连接（幂等）。
func (s *session) Close() {
	s.closed.Do(func() {
		s.sshSess.Close()
		s.client.Close()
	})
}

// exitMessage 构造 {"type":"exit","code":n} 控制帧。
func exitMessage(code int) []byte {
	return []byte(fmt.Sprintf(`{"type":"exit","code":%d}`, code))
}
