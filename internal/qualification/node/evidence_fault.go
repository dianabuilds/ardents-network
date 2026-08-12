package node

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

// runNodeEvidenceFault proves bounded Node fail-stop when its terminal evidence sink disappears.
func runNodeEvidenceFault(parent context.Context) Result {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/local/bin/ardents-node", "node", "--config", "/run/ardents/config.json")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	diagnostics := byteio.NewBuffer(16 << 10)
	command.Stderr = diagnostics
	if err := command.Start(); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	ready := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"state":"READY"`) {
			ready = true
			break
		}
	}
	if !ready {
		_ = command.Process.Kill()
		<-exited
		return Result{Verdict: "fail", Reason: "sacrificial Node did not reach READY before evidence failure"}
	}
	_ = stdout.Close()
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
		<-exited
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case waitErr := <-exited:
		if waitErr == nil || diagnostics.Overflowed() {
			return Result{Verdict: "fail", Reason: "Node did not fail closed on terminal evidence loss"}
		}
		return Result{Verdict: "pass", Reason: "terminal evidence loss stopped Node admission within two seconds"}
	case <-timer.C:
		_ = command.Process.Kill()
		<-exited
		return Result{Verdict: "fail", Reason: "terminal evidence loss exceeded the two-second fail-stop bound"}
	}
}
