package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ardents/internal/storage"
)

type transportKind uint8

var newSSHCommand = func(arguments []string) *exec.Cmd { return exec.Command("ssh", arguments...) }

const (
	transportHTTP transportKind = iota
	transportUnix
	transportSSHLegacy
	transportSSHStreamLocal
)

func controlTransport(cfg Config) (string, http.RoundTripper, transportKind, func() error) {
	parsed, _ := url.Parse(cfg.BaseURL)
	transport := &http.Transport{}
	if parsed.Scheme == "unix" {
		socketPath := parsed.Path
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		}
		return "http://ardents.local", transport, transportUnix, func() error { transport.CloseIdleConnections(); return nil }
	}
	if cfg.SSH != "" {
		if cfg.SSHOperatorSocket != "" {
			tunnel := newSSHStreamLocalTransport(cfg)
			return "http://ardents.local", tunnel, transportSSHStreamLocal, tunnel.Close
		}
		remoteAddress := parsed.Host
		transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
			return dialSSH(cfg, remoteAddress)
		}
		return cfg.BaseURL, transport, transportSSHLegacy, func() error { transport.CloseIdleConnections(); return nil }
	}
	return cfg.BaseURL, http.DefaultTransport, transportHTTP, func() error { return nil }
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
	return (&net.Dialer{}).DialContext(ctx, "unix", t.socket)
}

func (t *sshStreamLocalTransport) ensureStarted(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("SSH stream-local forwarding is closed")
	}
	if t.started {
		return t.startErr
	}
	t.started = true
	dir, err := os.MkdirTemp("", "ardents-ssh-")
	if err != nil {
		t.startErr = errors.New("create SSH forwarding state")
		return t.startErr
	}
	if err := storage.EnsurePrivateDir(dir); err != nil {
		_ = os.RemoveAll(dir)
		t.startErr = errors.New("protect SSH forwarding state")
		return t.startErr
	}
	t.tempDir = dir
	t.socket = filepath.Join(dir, "operator.sock")
	arguments := sshStreamLocalArguments(t.cfg, "operator.sock")
	command := newSSHCommand(arguments)
	command.Dir = dir
	command.Stdout = io.Discard
	command.Stderr = &boundedBuffer{limit: 4096}
	t.command = command
	if err := command.Start(); err != nil {
		t.startErr = errors.New("start SSH stream-local forwarding")
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
			t.startErr = ctx.Err()
			return t.startErr
		case <-t.done:
			t.startErr = errors.New("SSH stream-local forwarding exited before readiness")
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

func sshStreamLocalArguments(cfg Config, localSocket string) []string {
	arguments := []string{"-N", "-T", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-p", strconv.Itoa(cfg.SSHPort)}
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

func dialSSH(cfg Config, remoteAddress string) (net.Conn, error) {
	arguments := []string{"-T", "-o", "BatchMode=yes", "-p", strconv.Itoa(cfg.SSHPort)}
	if cfg.SSHIdentity != "" {
		arguments = append(arguments, "-o", "IdentitiesOnly=yes", "-i", cfg.SSHIdentity)
	}
	if cfg.SSHKnownHosts != "" {
		arguments = append(arguments, "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile="+cfg.SSHKnownHosts)
	}
	arguments = append(arguments, "-W", remoteAddress, cfg.SSH)
	command := exec.Command("ssh", arguments...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open ssh input: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open ssh output: %w", err)
	}
	connection := &commandConn{
		command: command, stdin: stdin, stdout: stdout, done: make(chan struct{}),
		local: commandAddress("local"), remote: commandAddress(cfg.SSH),
	}
	command.Stderr = &connection.stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start ssh transport: %w", err)
	}
	go connection.wait()
	return connection, nil
}

type commandConn struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  bytes.Buffer
	done    chan struct{}
	local   net.Addr
	remote  net.Addr

	waitOnce  sync.Once
	waitErr   error
	closeOnce sync.Once
	closeErr  error
}

func (c *commandConn) wait() {
	c.waitOnce.Do(func() {
		c.waitErr = c.command.Wait()
		close(c.done)
	})
}

func (c *commandConn) Read(buffer []byte) (int, error) {
	n, err := c.stdout.Read(buffer)
	if n > 0 || !errors.Is(err, io.EOF) {
		return n, err
	}
	<-c.done
	if c.waitErr != nil {
		detail := strings.TrimSpace(c.stderr.String())
		if detail != "" {
			return 0, fmt.Errorf("ssh transport failed: %s", detail)
		}
		return 0, fmt.Errorf("ssh transport failed: %w", c.waitErr)
	}
	return 0, io.EOF
}

func (c *commandConn) Write(buffer []byte) (int, error) { return c.stdin.Write(buffer) }

func (c *commandConn) Close() error {
	c.closeOnce.Do(func() {
		inputErr := c.stdin.Close()
		if c.command.Process != nil {
			_ = c.command.Process.Kill()
		}
		<-c.done
		c.closeErr = errors.Join(inputErr, c.stdout.Close())
	})
	return c.closeErr
}

func (c *commandConn) LocalAddr() net.Addr            { return c.local }
func (c *commandConn) RemoteAddr() net.Addr           { return c.remote }
func (*commandConn) SetDeadline(time.Time) error      { return nil }
func (*commandConn) SetReadDeadline(time.Time) error  { return nil }
func (*commandConn) SetWriteDeadline(time.Time) error { return nil }

type commandAddress string

func (a commandAddress) Network() string { return "ssh" }
func (a commandAddress) String() string  { return string(a) }
