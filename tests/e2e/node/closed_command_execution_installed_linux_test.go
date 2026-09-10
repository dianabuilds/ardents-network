//go:build linux && text_worker_installed

package state_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type installedCommandOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

func (output *installedCommandOutput) Write(data []byte) (int, error) {
	if len(data) > output.limit-output.buffer.Len() {
		output.overflow = true
		output.cancel()
		return 0, errors.New("qualification output limit exceeded")
	}
	return output.buffer.Write(data)
}
func installedCommandExec(ctx context.Context, input []byte, name string, args ...string) ([]byte, []byte, error) {
	return installedCommandProcess(ctx, input, nil, name, args...)
}
func installedCommandExecAs(ctx context.Context, input []byte, uid, gid int, name string, args ...string) ([]byte, []byte, error) {
	return installedCommandProcess(ctx, input, &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}, name, args...)
}
func installedCommandProcess(ctx context.Context, input []byte, credential *syscall.Credential, name string, args ...string) ([]byte, []byte, error) {
	owned, cancel := context.WithCancel(ctx)
	defer cancel()
	output := &installedCommandOutput{limit: 5 << 20, cancel: cancel}
	diagnostic := &installedCommandOutput{limit: 64 << 10, cancel: cancel}
	command := exec.CommandContext(owned, name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Credential: credential}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.Stdin, command.Stdout, command.Stderr = bytes.NewReader(input), output, diagnostic
	command.WaitDelay = 5 * time.Second
	err := command.Run()
	if command.Process != nil {
		groupErr := awaitInstalledCommandGroup(command.Process.Pid)
		err = errors.Join(err, groupErr)
	}
	if output.overflow || diagnostic.overflow {
		err = errors.Join(err, errors.New("qualification output overflow"))
	}
	return output.buffer.Bytes(), diagnostic.buffer.Bytes(), errors.Join(err, owned.Err())
}

func TestInstalledCommandOutputOverflowCancelsAndJoins(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	output, _, err := installedCommandExec(ctx, nil, "head", "-c", "6000000", "/dev/zero")
	if err == nil || !strings.Contains(err.Error(), "qualification output overflow") || len(output) > 5<<20 {
		t.Fatalf("output overflow was not bounded and joined: %d bytes, %v", len(output), err)
	}
}

// Commands are trusted fixed pipelines. This owns their process group, not
// confinement of arbitrary programs capable of creating another session.
func awaitInstalledCommandGroup(pid int) error {
	if err := syscall.Kill(-pid, 0); errors.Is(err, syscall.ESRCH) {
		return nil
	}
	cleanupErr := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(cleanupErr, syscall.ESRCH) {
		cleanupErr = nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(-pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return cleanupErr
		}
		if err != nil {
			return errors.Join(cleanupErr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.Join(cleanupErr, errors.New("qualification command group did not terminate"))
}

func TestInstalledCommandCancellationJoinsPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	output, _, err := installedCommandExec(ctx, nil, "bash", "-c", `sleep 60 & printf '%s\n' "$!"; wait`)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(output)))
	if parseErr != nil || pid <= 0 {
		t.Fatalf("missing child PID: %q", output)
	}
	if probe := syscall.Kill(pid, 0); !errors.Is(probe, syscall.ESRCH) {
		t.Fatalf("cancelled command retained child %d: %v", pid, probe)
	}
	if strings.Contains(err.Error(), "group did not terminate") {
		t.Fatal(err)
	}
}

func TestInstalledCommandUserOwnsPresentationPipes(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root must select the credential-drop command profile")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	output, diagnostic, err := installedCommandExecAs(ctx, []byte("presentation\n"), 65534, 65534,
		"bash", "-o", "pipefail", "-c", `cat | "$@" | cat`, "command-stdio", "bash", "-c",
		`exec 4</proc/self/fd/0; exec 5>/proc/self/fd/1; id -u >&5; id -G >&5; cat <&4 >&5`)
	if err != nil || string(output) != "65534\n65534\npresentation\n" {
		t.Fatalf("user-owned stdio/groups: %q / %s / %v", output, diagnostic, err)
	}
}
