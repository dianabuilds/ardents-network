//go:build ignore

package main

import (
	"bytes"
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
	"syscall"
	"time"
)

type child struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stderr     bytes.Buffer
	wait       chan error
	transcript []byte
}

type processObservation struct {
	PID          int    `json:"pid"`
	Capabilities string `json:"capabilities"`
	Descendants  int    `json:"descendants"`
	FDs          int    `json:"fds"`
	Sockets      int    `json:"sockets"`
	RSSKiB       int64  `json:"rss_kib"`
}

func startChild(path string, env map[string]string, method, kind string) (*child, readiness, error) {
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
	c := &child{cmd: cmd, stdin: stdin, wait: make(chan error, 1)}
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
		return c, result.ready, nil
	case <-time.After(5 * time.Second):
		_ = c.forceStop()
		return c, readiness{}, fmt.Errorf("%s readiness timed out", method)
	}
}

func (c *child) stop() (string, time.Duration, error) {
	started := time.Now()
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
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	select {
	case <-c.wait:
		return nil
	case <-time.After(time.Second):
		return errors.New("forced child reap timed out")
	}
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
