//go:build installation_native

package installation

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Normal cancellation must also wait the exact helper before returning. This
// physical fixture supplies no account creation or successful authority.
func TestInstallationNativeAccountCommandCancellationJoinsChild(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reader, writer, err := os.Pipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer reader.Close()
	if err := reader.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		cancel()
		_ = writer.Close()
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "/bin/sh", "-c", `printf '%s\n' "$$"; exec /usr/bin/sleep 30`)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.Stdout, command.Stderr = writer, io.Discard
	command.WaitDelay = 5 * time.Second
	finished := make(chan error, 1)
	joined := false
	go func() { finished <- errors.Join(runOriginalAccountCommand(command), writer.Close()) }()
	defer func() {
		cancel()
		if !joined {
			<-finished
		}
	}()
	line, err := bufio.NewReader(reader).ReadString('\n')
	pid, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || parseErr != nil || pid <= 0 {
		t.Fatal("original account helper did not report its identity", line, err, parseErr)
	}
	original, err := os.Open("/proc/" + strconv.Itoa(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	cancel()
	err = <-finished
	joined = true
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatal("cancelled helper result lost", err)
	}
	status, ok := exited.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("original helper was not killed and waited", err)
	}
	fd, err := syscall.Openat(int(original.Fd()), "stat", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if fd >= 0 {
		_ = syscall.Close(fd)
	}
	if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ESRCH) {
		t.Fatal("cancelled helper remains after Run", err)
	}
}

// Only the isolated monitor becomes a subreaper. It retains and waits the exact
// physical child after fatal caller death; a zombie is not a joined child.
// The shell/sleep fixture supplies no account, Release or manager authority.
func TestInstallationNativeAccountCommandFatalCallerJoinsChild(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestInstallationAccountCommandMonitorChild$", "-test.timeout=10s")
	command.Env = append(os.Environ(), "ARDENTS_INSTALLATION_ACCOUNT_COMMAND_ROLE=monitor")
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	output, err := command.CombinedOutput()
	runtime.UnlockOSThread()
	if err != nil {
		t.Fatalf("original account-command fatal-caller lifetime: %v: %s", err, output)
	}
}

func TestInstallationAccountCommandCallerChild(t *testing.T) {
	if os.Getenv("ARDENTS_INSTALLATION_ACCOUNT_COMMAND_ROLE") != "caller" {
		t.Fatal("invalid environment: account-command caller requires its selected monitor")
	}
	ready := os.NewFile(3, "account-command-ready")
	defer ready.Close()
	command := exec.CommandContext(t.Context(), "/bin/sh", "-c", `printf '%s\n' "$$" >&3; exec /usr/bin/sleep 30`)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.ExtraFiles = []*os.File{ready}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	command.WaitDelay = 5 * time.Second
	if err := runOriginalAccountCommand(command); err != nil {
		t.Fatal(err)
	}
	t.Fatal("original caller unexpectedly outlived its selected fatal stop")
}

func TestInstallationAccountCommandMonitorChild(t *testing.T) {
	if os.Getenv("ARDENTS_INSTALLATION_ACCOUNT_COMMAND_ROLE") != "monitor" {
		t.Fatal("invalid environment: account-command monitor requires its selected parent")
	}
	// PR_SET_CHILD_SUBREAPER applies only to this disposable test process.
	if _, _, err := syscall.Syscall6(syscall.SYS_PRCTL, 36, 1, 0, 0, 0, 0); err != 0 {
		t.Fatal("invalid environment: isolated child-reaping custody unavailable", err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if err := reader.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	caller := exec.CommandContext(t.Context(), executable, "-test.run=^TestInstallationAccountCommandCallerChild$", "-test.timeout=10s")
	caller.Env = append(os.Environ(), "ARDENTS_INSTALLATION_ACCOUNT_COMMAND_ROLE=caller")
	caller.ExtraFiles = []*os.File{writer}
	caller.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := caller.Start(); err != nil {
		t.Fatal(err)
	}
	callerJoined := false
	defer func() {
		if !callerJoined {
			_ = caller.Process.Kill()
			_ = caller.Wait()
		}
	}()
	line, err := bufio.NewReader(reader).ReadString('\n')
	pid, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || parseErr != nil || pid <= 0 {
		t.Fatal("original helper did not report its identity", line, err, parseErr)
	}
	original, err := os.Open("/proc/" + strconv.Itoa(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	helperJoined := false
	defer func() {
		if !helperJoined {
			var status syscall.WaitStatus
			// Until reaped this exact child PID cannot be reused. Refuse a
			// signal if it is no longer our independently retained child.
			if got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil); err == nil && got == 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				_, _ = syscall.Wait4(pid, &status, 0, nil)
			}
		}
	}()
	if err := caller.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	err = caller.Wait()
	callerJoined = true
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatal("fatal caller result was lost", err)
	}
	status, ok := exited.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("original caller was not killed and joined", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var terminal syscall.WaitStatus
		got, err := syscall.Wait4(pid, &terminal, syscall.WNOHANG, nil)
		if err != nil {
			t.Fatal("original helper reaping custody lost", err)
		}
		if got == pid {
			helperJoined = true
			if !terminal.Signaled() || terminal.Signal() != syscall.SIGKILL {
				t.Fatal("original helper did not retain kernel parent-death termination", terminal)
			}
			fd, err := syscall.Openat(int(original.Fd()), "stat", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
			if fd >= 0 {
				_ = syscall.Close(fd)
			}
			if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ESRCH) {
				t.Fatal("original helper remains after its exact wait", err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("original account-command child survives fatal caller death")
}
