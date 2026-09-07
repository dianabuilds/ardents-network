//go:build ignore

// Disposable R-152 process-boundary probe; intentionally hostile to its sandbox.
package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
)

type result struct {
	Name    string `json:"name"`
	Allowed bool   `json:"allowed"`
	Detail  string `json:"detail,omitempty"`
}

func main() {
	base := os.Getenv("ARDENTS_PROBE_ROOT")
	if !strings.HasPrefix(base, "/tmp/ardents-r152.") {
		fmt.Fprintln(os.Stderr, "invalid probe root")
		os.Exit(2)
	}
	mode := "sandbox"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	checks := []result{}
	test := func(name string, fn func() error) {
		err := fn()
		r := result{Name: name, Allowed: err == nil}
		if err != nil {
			r.Detail = err.Error()
		}
		checks = append(checks, r)
	}
	test("ipv4_listener", func() error {
		l, e := net.Listen("tcp4", "127.0.0.1:0")
		if e == nil {
			l.Close()
		}
		return e
	})
	test("ipv6_listener", func() error {
		l, e := net.Listen("tcp6", "[::1]:0")
		if e == nil {
			l.Close()
		}
		return e
	})
	test("udp_dns_socket", func() error {
		c, e := net.Dial("udp4", "192.0.2.1:53")
		if e == nil {
			c.Close()
		}
		return e
	})
	test("host_sentinel", func() error { _, e := os.ReadFile(base + "/outside/sentinel"); return e })
	test("host_ipc", func() error {
		c, e := net.Dial("unix", "/run/systemd/private")
		if e == nil {
			c.Close()
		}
		return e
	})
	test("root_write", func() error {
		return os.WriteFile(base+"/outside/probe-write", []byte("probe"), 0600)
	})

	if mode != "child" {
		out, e := exec.Command(os.Args[0], "child").CombinedOutput()
		if e != nil {
			fmt.Fprintln(os.Stderr, string(out), e)
			os.Exit(2)
		}
		var child struct {
			Checks []result `json:"checks"`
		}
		if e = json.Unmarshal(out, &child); e != nil {
			fmt.Fprintln(os.Stderr, string(out), e)
			os.Exit(2)
		}
		for _, r := range child.Checks {
			r.Name = "child_" + r.Name
			checks = append(checks, r)
		}
	}
	test("mount_namespace", func() error { return unix.Unshare(unix.CLONE_NEWNET) })
	input, _ := io.ReadAll(io.LimitReader(os.Stdin, 256))
	status, _ := os.ReadFile("/proc/self/status")
	restrictions := []string{}
	for _, line := range strings.Split(string(status), "\n") {
		for _, p := range []string{"NoNewPrivs:", "Seccomp:", "CapEff:"} {
			if strings.HasPrefix(line, p) {
				restrictions = append(restrictions, line)
			}
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": mode, "stdin_echo": string(input), "uid": os.Getuid(), "checks": checks, "kernel_status": restrictions})
}
