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
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func controlTransport(cfg Config) (string, http.RoundTripper) {
	parsed, _ := url.Parse(cfg.BaseURL)
	transport := &http.Transport{}
	if parsed.Scheme == "unix" {
		socketPath := parsed.Path
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		}
		return "http://ardents.local", transport
	}
	if cfg.SSH != "" {
		remoteAddress := parsed.Host
		transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
			return dialSSH(cfg, remoteAddress)
		}
		return cfg.BaseURL, transport
	}
	return cfg.BaseURL, http.DefaultTransport
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
