//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type child struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stderr     boundedBuffer
	wait       chan error
	transcript []byte
	stdoutDone chan struct{}
	stopMu     sync.Mutex
	stopped    bool
	stopRung   string
	stopTime   time.Duration
	stopErr    error
}

type processObservation struct {
	PID          int    `json:"pid"`
	Capabilities string `json:"capabilities"`
	Descendants  int    `json:"descendants"`
	FDs          int    `json:"fds"`
	Sockets      int    `json:"sockets"`
	RSSKiB       int64  `json:"rss_kib"`
}

func startChild(path string, env map[string]string, method, kind string, deadline time.Time) (*child, readiness, error) {
	if !time.Now().Before(deadline) {
		return nil, readiness{}, fmt.Errorf("%s startup deadline expired", method)
	}
	cmd := exec.Command(path)
	cmd.Env = []string{"LANG=C", "GODEBUG=netdns=go+2"}
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, readiness{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, readiness{}, err
	}
	c := &child{cmd: cmd, stdin: stdin, wait: make(chan error, 1),
		stderr: boundedBuffer{limit: maxControlTranscript}}
	cmd.Stderr = &c.stderr
	if err := cmd.Start(); err != nil {
		return nil, readiness{}, err
	}
	go func() { c.wait <- cmd.Wait() }()
	type readyResult struct {
		ready      readiness
		transcript []byte
		err        error
	}
	ready := make(chan readyResult, 1)
	go func() {
		got, transcript, err := readReadiness(stdout, method, kind)
		ready <- readyResult{got, transcript, err}
	}()
	select {
	case result := <-ready:
		c.transcript = result.transcript
		if result.err != nil {
			_ = c.forceStop()
			return c, result.ready, result.err
		}
		if c.stderr.Exceeded() || len(c.transcript)+c.stderr.Len() > maxControlTranscript {
			_ = c.forceStop()
			return c, result.ready, errors.New("combined control output limit exceeded")
		}
		c.stdoutDone = make(chan struct{})
		go func() {
			_, _ = io.Copy(io.Discard, stdout)
			close(c.stdoutDone)
		}()
		return c, result.ready, nil
	case <-time.After(time.Until(deadline)):
		_ = c.forceStop()
		return c, readiness{}, fmt.Errorf("%s readiness timed out", method)
	}
}

func (c *child) stop(requested string) (string, time.Duration, error) {
	c.stopMu.Lock()
	defer c.stopMu.Unlock()
	if c.stopped {
		return c.stopRung, c.stopTime, c.stopErr
	}
	c.stopped = true
	c.stopRung, c.stopTime, c.stopErr = c.stopOnce(requested)
	if c.stopErr == nil && c.stdoutDone != nil {
		select {
		case <-c.stdoutDone:
		case <-time.After(100 * time.Millisecond):
			c.stopErr = errors.New("stdout drain did not stop")
		}
	}
	return c.stopRung, c.stopTime, c.stopErr
}

func (c *child) stopOnce(requested string) (string, time.Duration, error) {
	started := time.Now()
	defer c.stdin.Close()
	if requested == "sigterm" {
		if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return "sigterm-signal-failed", time.Since(started), err
		}
		select {
		case <-c.wait:
			return "sigterm", time.Since(started), nil
		case <-time.After(1500 * time.Millisecond):
			return "sigterm-timeout", time.Since(started), errors.New("SIGTERM did not reap child")
		}
	}
	if requested == "sigkill" {
		if err := c.cmd.Process.Kill(); err != nil {
			return "sigkill-signal-failed", time.Since(started), err
		}
		select {
		case <-c.wait:
			return "sigkill", time.Since(started), nil
		case <-time.After(500 * time.Millisecond):
			return "unreaped", time.Since(started), fmt.Errorf("pid %d was not reaped", c.cmd.Process.Pid)
		}
	}
	_ = c.stdin.Close()
	select {
	case err := <-c.wait:
		return "stdin", time.Since(started), exitOK(err)
	case <-time.After(1500 * time.Millisecond):
	}
	_ = c.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-c.wait:
		return "sigterm", time.Since(started), exitOK(err)
	case <-time.After(1500 * time.Millisecond):
	}
	_ = c.cmd.Process.Kill()
	select {
	case <-c.wait:
		return "sigkill", time.Since(started), nil
	case <-time.After(500 * time.Millisecond):
		return "unreaped", time.Since(started), fmt.Errorf("pid %d was not reaped", c.cmd.Process.Pid)
	}
}

func (c *child) forceStop() error {
	_, _, err := c.stop("sigkill")
	return err
}

func exitOK(err error) error {
	if err == nil {
		return nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 0 {
		return nil
	}
	return err
}

func observeProcess(pid int) (processObservation, error) {
	root := filepath.Join("/proc", strconv.Itoa(pid))
	status, err := os.ReadFile(filepath.Join(root, "status"))
	if err != nil {
		return processObservation{}, err
	}
	obs := processObservation{PID: pid}
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "CapEff:":
			obs.Capabilities = fields[1]
		case "VmRSS:":
			obs.RSSKiB, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}
	children, _ := os.ReadFile(filepath.Join(root, "task", strconv.Itoa(pid), "children"))
	obs.Descendants = len(strings.Fields(string(children)))
	entries, err := os.ReadDir(filepath.Join(root, "fd"))
	if err != nil {
		return processObservation{}, err
	}
	obs.FDs = len(entries)
	for _, entry := range entries {
		target, _ := os.Readlink(filepath.Join(root, "fd", entry.Name()))
		if strings.HasPrefix(target, "socket:[") {
			obs.Sockets++
		}
	}
	return obs, nil
}

func (c *child) transcriptHash() string {
	digest := sha256.Sum256(c.transcript)
	return hex.EncodeToString(digest[:])
}
