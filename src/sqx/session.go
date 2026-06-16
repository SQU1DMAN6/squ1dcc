package sqx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Session manages a persistent SQX module process for stateful communication.
// The SQX process stays alive and receives JSON requests on stdin,
// sending JSON responses on stdout.
type Session struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Scanner
	mu       sync.Mutex
	closed   bool
	lastCall time.Time
	timeout  time.Duration
}

// SessionConfig holds configuration for starting an SQX session.
type SessionConfig struct {
	// Path to the SQX executable module.
	Path string
	// Optional working directory. Defaults to the module's directory.
	Dir string
	// Timeout per call. 0 means no timeout.
	CallTimeout time.Duration
	// Additional environment variables.
	Env map[string]string
}

// NewSession starts a persistent SQX module process.
// The process is started with the `--session` flag (or via stdin protocol detection)
// and communicates using JSON-over-stdio.
func NewSession(cfg SessionConfig) (*Session, error) {
	absPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("could not resolve SQX path %q: %w", cfg.Path, err)
	}

	workDir := cfg.Dir
	if workDir == "" {
		workDir = filepath.Dir(absPath)
	}

	// Start the SQX process with the session flag
	cmd := exec.Command(absPath, "--session")
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("could not create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("could not create stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("could not create stderr pipe: %w", err)
	}

	// Set environment variables if provided
	if len(cfg.Env) > 0 {
		env := os.Environ()
		for k, v := range cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("could not start SQX session for %q: %w", absPath, err)
	}

	// Read stderr asynchronously (log it, don't block)
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			// Log stderr from the SQX module; in production this could be
			// redirected to a logger.
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				fmt.Fprintf(os.Stderr, "[sqx-session:%s] %s\n", filepath.Base(absPath), line)
			}
		}
	}()

	sess := &Session{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewScanner(stdoutPipe),
		lastCall: time.Now(),
		timeout:  cfg.CallTimeout,
	}

	// Verify the session is alive by sending a ping
	if err := sess.ping(); err != nil {
		sess.Close()
		return nil, fmt.Errorf("SQX session %q did not respond to ping: %w", absPath, err)
	}

	return sess, nil
}

// ping sends a health-check request and waits for a response.
func (s *Session) ping() error {
	_, err := s.sendRequest(map[string]interface{}{
		"cmd": "ping",
	})
	return err
}

// Call invokes a function in the SQX session and returns the structured result.
// The result is a Hash with keys: "ok" (bool), "value" (any), "error" (string|null).
func (s *Session) Call(fn string, args []string) (map[string]interface{}, error) {
	request := map[string]interface{}{
		"cmd":  "call",
		"fn":   fn,
		"args": args,
	}
	return s.sendRequest(request)
}

// sendRequest sends a JSON request and reads the JSON response.
func (s *Session) sendRequest(request map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("session is closed")
	}

	// Encode and send the request
	reqData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := s.stdin.Write(reqData); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	if _, err := s.stdin.Write([]byte("\n")); err != nil {
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}

	s.lastCall = time.Now()

	// Read the response
	if !s.stdout.Scan() {
		if err := s.stdout.Err(); err != nil {
			return nil, fmt.Errorf("error reading response: %w", err)
		}
		return nil, fmt.Errorf("session closed unexpectedly")
	}

	line := s.stdout.Text()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w: %q", err, line)
	}

	return result, nil
}

// Close terminates the SQX session process.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Send shutdown command
	s.stdin.Write([]byte(`{"cmd":"shutdown"}` + "\n"))

	// Close stdin to signal EOF
	s.stdin.Close()

	// Wait for process to exit with a timeout
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.cmd.Process.Kill()
		<-done
	}

	return nil
}

// IsClosed returns whether the session has been closed.
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// SessionPool manages a pool of reusable SQX sessions.
type SessionPool struct {
	mu       sync.Mutex
	sessions []*Session
	cfg      SessionConfig
	maxSize  int
}

// NewSessionPool creates a pool of SQX sessions.
func NewSessionPool(cfg SessionConfig, maxSize int) *SessionPool {
	return &SessionPool{
		cfg:     cfg,
		maxSize: maxSize,
	}
}

// Acquire gets a session from the pool, creating one if needed.
func (p *SessionPool) Acquire() (*Session, error) {
	p.mu.Lock()

	// Try to reuse an existing session
	for len(p.sessions) > 0 {
		sess := p.sessions[len(p.sessions)-1]
		p.sessions = p.sessions[:len(p.sessions)-1]
		p.mu.Unlock()

		if !sess.IsClosed() {
			return sess, nil
		}
		// Discard closed sessions
	}

	p.mu.Unlock()

	// Create a new session
	return NewSession(p.cfg)
}

// Release returns a session to the pool.
func (p *SessionPool) Release(sess *Session) {
	if sess == nil || sess.IsClosed() {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.sessions) >= p.maxSize {
		sess.Close()
		return
	}

	p.sessions = append(p.sessions, sess)
}

// CloseAll terminates all sessions in the pool.
func (p *SessionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sess := range p.sessions {
		sess.Close()
	}
	p.sessions = nil
}