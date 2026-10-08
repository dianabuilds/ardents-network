//go:build installation_native

package systemd

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This is a physical subprocess fixture, not a successful manager or runtime.
// Cancellation must join the exact child while its original proc descriptor is
// retained; a returned error or cancelled context alone is insufficient.
func TestInstallationNativeManagerCommandCancellationJoinsChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	reader, writer, err := os.Pipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer reader.Close()
	finished := make(chan error, 1)
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-finished
		}
	}()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", `printf '%s\n' "$$"; exec /usr/bin/sleep 30`)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.Stdout, command.Stderr = writer, io.Discard
	command.WaitDelay = 5 * time.Second
	go func() {
		err := runOriginalManagerCommand(command)
		finished <- errors.Join(err, writer.Close())
	}()
	line, err := bufio.NewReader(reader).ReadString('\n')
	pid, parseErr := strconv.ParseUint(strings.TrimSpace(line), 10, 32)
	if err != nil || parseErr != nil || pid == 0 {
		t.Fatal("original child did not report its identity", line, err, parseErr)
	}
	path := "/proc/" + strconv.FormatUint(pid, 10)
	original, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	cancel()
	err = <-finished
	joined = true
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatal("cancellation lost original subprocess result", err)
	}
	status, ok := exited.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("original child was not killed and waited", exited)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Run returned before original child disappeared", err)
	}
}

// These refusal calls execute no manager operation and supply no live authority.
func TestInstallationNativeManagerRefusesMissingOrCancelledCaller(t *testing.T) {
	if unit, service, err := Endpoint(nil); !errors.Is(err, ErrInput) || unit != nil || service != nil {
		t.Fatal("missing caller returned manager facts", unit, service, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if unit, service, err := Endpoint(ctx); !errors.Is(err, context.Canceled) || unit != nil || service != nil {
		t.Fatal("cancelled caller returned Endpoint facts", unit, service, err)
	}
	if unit, socket, err := Activation(ctx, "reader"); !errors.Is(err, context.Canceled) || unit != nil || socket != nil {
		t.Fatal("cancelled caller returned socket facts", unit, socket, err)
	}
	if unit, socket, err := Activation(t.Context(), "foreign"); !errors.Is(err, ErrInput) || unit != nil || socket != nil {
		t.Fatal("foreign object admitted", unit, socket, err)
	}
}

// Every public fixed mechanism refuses before opening a subprocess when the
// original caller is missing or already cancelled. No system manager is faked.
func TestInstallationNativeControlRefusesBeforeSubprocess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	observations := []struct {
		name string
		run  func(context.Context) (string, error)
	}{
		{"Endpoint state", func(ctx context.Context) (string, error) { return State(ctx, "ardents-endpoint.service") }},
		{"socket state", func(ctx context.Context) (string, error) { return State(ctx, "ardents-text-reader.socket") }},
		{"instances", Instances}, {"units", Units}, {"version", Version},
	}
	for _, observation := range observations {
		if body, err := observation.run(nil); body != "" || !errors.Is(err, ErrInput) {
			t.Fatal(observation.name, "missing caller admitted", body, err)
		}
		if body, err := observation.run(ctx); body != "" || !errors.Is(err, context.Canceled) {
			t.Fatal(observation.name, "cancelled caller admitted", body, err)
		}
	}
	for _, effect := range []func(context.Context) error{Reload, Start, Stop} {
		if err := effect(nil); !errors.Is(err, ErrInput) {
			t.Fatal("missing caller admitted effect", err)
		}
		if err := effect(ctx); !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled caller admitted effect", err)
		}
	}
	for _, unit := range []string{"", "foreign.service", "ardents-endpoint.service\nforeign.service", "ardents-text-reader@1.service"} {
		if body, err := State(t.Context(), unit); body != "" || !errors.Is(err, ErrInput) {
			t.Fatal("foreign unit admitted", body, err)
		}
	}
}

func TestInstallationNativeManagerOutputRetainsFiniteBound(t *testing.T) {
	var output boundedManagerOutput
	if written, err := output.Write(make([]byte, (64<<10)-1)); err != nil || written != (64<<10)-1 {
		t.Fatal(written, err)
	}
	if written, err := output.Write([]byte("x")); err != nil || written != 1 {
		t.Fatal(written, err)
	}
	if written, err := output.Write([]byte("foreign")); written != 0 || !errors.Is(err, ErrUnavailable) || output.body.Len() != 64<<10 {
		t.Fatal("oversized manager output accepted or retained", written, err)
	}
}
