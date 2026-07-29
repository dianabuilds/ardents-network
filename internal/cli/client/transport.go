package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ardents/internal/storage"
)

type transportKind uint8

var newSSHCommand = func(arguments []string) *exec.Cmd { return exec.Command("ssh", arguments...) }

// ErrSSHHostKeyMismatch is the stable transport classification for an
// OpenSSH host-key pin rejection. It never contains host or pin material.
var ErrSSHHostKeyMismatch = errors.New("SSH host key mismatch")

// ErrSSHTunnelFailure is the stable transport classification for a local
// OpenSSH forwarding failure. It never contains target or socket material.
var ErrSSHTunnelFailure = errors.New("SSH tunnel failure")

// ErrSSHTunnelTimeout is the stable transport classification for a local
// OpenSSH forwarding timeout. It never contains target or socket material.
var ErrSSHTunnelTimeout = errors.New("SSH tunnel timeout")

const (
	transportUnix transportKind = iota
	transportSSHStreamLocal
)

func controlTransport(cfg Config) (string, http.RoundTripper, transportKind, func() error, error) {
	if cfg.SSH != "" {
		if strings.HasPrefix(cfg.SSH, "-") || strings.ContainsAny(cfg.SSH, " \t\r\n") {
			return "", nil, 0, nil, errors.New("invalid SSH target")
		}
		if !path.IsAbs(cfg.SSHOperatorSocket) || strings.ContainsAny(cfg.SSHOperatorSocket, "\x00:\r\n") {
			return "", nil, 0, nil, errors.New("SSH transport requires an absolute remote Operator Unix socket")
		}
		tunnel := newSSHStreamLocalTransport(cfg)
		return "http://ardents.local", tunnel, transportSSHStreamLocal, tunnel.Close, nil
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme != "unix" || parsed.Path == "" {
		return "", nil, 0, nil, errors.New("Operator transport requires a protected Unix socket or SSH stream-local forwarding")
	}
	socketPath := parsed.Path
	if parsed.Host != "" {
		socketPath = parsed.Host + parsed.Path
		if !filepath.IsAbs(socketPath) {
			return "", nil, 0, nil, errors.New("Operator transport requires a protected Unix socket or SSH stream-local forwarding")
		}
	}
	transport := &http.Transport{}
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
	return "http://ardents.local", transport, transportUnix, func() error { transport.CloseIdleConnections(); return nil }, nil
}

type sshStreamLocalTransport struct {
	cfg       Config
	transport *http.Transport

	mu        sync.Mutex
	started   bool
	closed    bool
	startErr  error
	command   *exec.Cmd
	done      chan struct{}
	tempDir   string
	socket    string
	closeOnce sync.Once
	closeErr  error
}

func newSSHStreamLocalTransport(cfg Config) *sshStreamLocalTransport {
	tunnel := &sshStreamLocalTransport{cfg: cfg}
	tunnel.transport = &http.Transport{DialContext: tunnel.dialContext}
	return tunnel
}

func (t *sshStreamLocalTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(request)
}

func (t *sshStreamLocalTransport) dialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	if err := t.ensureStarted(ctx); err != nil {
		return nil, err
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", t.socket)
	if err != nil {
		return nil, fmt.Errorf("%w: local forwarding socket unavailable", ErrSSHTunnelFailure)
	}
	return connection, nil
}

func (t *sshStreamLocalTransport) ensureStarted(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return fmt.Errorf("%w: SSH stream-local forwarding is closed", ErrSSHTunnelFailure)
	}
	if t.started {
		return t.startErr
	}
	t.started = true
	dir, err := os.MkdirTemp("", "ardents-ssh-")
	if err != nil {
		t.startErr = fmt.Errorf("%w: create SSH forwarding state", ErrSSHTunnelFailure)
		return t.startErr
	}
	if err := storage.EnsurePrivateDir(dir); err != nil {
		_ = os.RemoveAll(dir)
		t.startErr = fmt.Errorf("%w: protect SSH forwarding state", ErrSSHTunnelFailure)
		return t.startErr
	}
	t.tempDir = dir
	t.socket = filepath.Join(dir, "operator.sock")
	arguments := sshStreamLocalArguments(t.cfg, "operator.sock")
	command := newSSHCommand(arguments)
	command.Dir = dir
	command.Stdout = io.Discard
	stderr := &boundedBuffer{limit: 4096}
	command.Stderr = stderr
	t.command = command
	if err := command.Start(); err != nil {
		t.startErr = fmt.Errorf("%w: start SSH stream-local forwarding", ErrSSHTunnelFailure)
		_ = os.RemoveAll(dir)
		return t.startErr
	}
	t.done = make(chan struct{})
	done := t.done
	go func() {
		_ = command.Wait()
		close(done)
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.stopLocked()
			t.startErr = fmt.Errorf("%w: %v", ErrSSHTunnelTimeout, ctx.Err())
			return t.startErr
		case <-t.done:
			if isSSHHostKeyMismatch(stderr.String()) {
				t.startErr = ErrSSHHostKeyMismatch
			} else {
				t.startErr = fmt.Errorf("%w: SSH stream-local forwarding exited before readiness", ErrSSHTunnelFailure)
			}
			_ = os.RemoveAll(dir)
			return t.startErr
		case <-ticker.C:
			info, statErr := os.Lstat(t.socket)
			if statErr == nil && info.Mode()&os.ModeSocket != 0 {
				return nil
			}
		}
	}
}

func isSSHHostKeyMismatch(stderr string) bool {
	stderr = strings.ToLower(stderr)
	return strings.Contains(stderr, "remote host identification has changed") ||
		strings.Contains(stderr, "host key verification failed") ||
		strings.Contains(stderr, "offending ") && strings.Contains(stderr, "host key")
}

func sshStreamLocalArguments(cfg Config, localSocket string) []string {
	arguments := []string{
		"-F", "none", "-N", "-T",
		"-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes",
		"-o", "GlobalKnownHostsFile=none",
		"-p", strconv.Itoa(cfg.SSHPort),
	}
	if cfg.SSHIdentity != "" {
		arguments = append(arguments, "-o", "IdentitiesOnly=yes", "-i", cfg.SSHIdentity)
	}
	if cfg.SSHKnownHosts != "" {
		arguments = append(arguments, "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile="+cfg.SSHKnownHosts)
	}
	arguments = append(arguments, "-L", localSocket+":"+cfg.SSHOperatorSocket, cfg.SSH)
	return arguments
}

func (t *sshStreamLocalTransport) stopLocked() error {
	if t.command != nil && t.command.Process != nil {
		_ = t.command.Process.Kill()
	}
	if t.done != nil {
		<-t.done
	}
	if t.tempDir != "" {
		return os.RemoveAll(t.tempDir)
	}
	return nil
}

func (t *sshStreamLocalTransport) Close() error {
	t.closeOnce.Do(func() {
		t.transport.CloseIdleConnections()
		t.mu.Lock()
		defer t.mu.Unlock()
		t.closed = true
		if t.started && t.command != nil {
			t.closeErr = t.stopLocked()
		} else if t.tempDir != "" {
			t.closeErr = os.RemoveAll(t.tempDir)
		}
	})
	return t.closeErr
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(value) < remaining {
			remaining = len(value)
		}
		b.data = append(b.data, value[:remaining]...)
	}
	return len(value), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}
