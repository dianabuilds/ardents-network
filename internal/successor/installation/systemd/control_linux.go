package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// State reads bounded textual facts for one of the three fixed Installation
// units. The caller retains phase/configuration policy and the observation order.
func State(ctx context.Context, unit string) (string, error) {
	properties := "--property=LoadState,ActiveState,SubState,FragmentPath,DropInPaths"
	switch unit {
	case "ardents-endpoint.service":
		properties += ",MainPID"
	case "ardents-text-reader.socket", "ardents-text-publisher.socket":
	default:
		return "", ErrInput
	}
	return control(ctx, "--no-pager", "show", properties, unit)
}

// Instances reads only the two fixed worker instance families. Empty output
// supplies no kernel join, process binding or startup authority.
func Instances(ctx context.Context) (string, error) {
	return control(ctx, "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-text-reader@*.service", "ardents-text-publisher@*.service")
}

// Units reads only the Installation unit inventory. Presence or absence grants
// no authority to create accounts, install files or stop an invocation.
func Units(ctx context.Context) (string, error) {
	return control(ctx, "--no-pager", "--no-legend", "--plain", "--all", "list-units", "ardents-endpoint.service", "ardents-text-*")
}

// Reload executes only daemon-reload after the owning transaction's admission.
func Reload(ctx context.Context) error {
	_, err := control(ctx, "daemon-reload")
	return err
}

// Stop requests the fixed sockets and Endpoint to stop, in that order. A command
// result does not establish physical join or permit releasing original pins.
// The caller alone chooses an admitted original or cleanup context.
func Stop(ctx context.Context) error {
	_, err := control(ctx, "--system", "--no-ask-password", "--no-pager", "stop", "ardents-text-reader.socket", "ardents-text-publisher.socket", "ardents-endpoint.service")
	return err
}

// Start requests only the fixed Endpoint. Its authenticated unit owns socket
// dependencies. The caller admits this attempt before the effect and retains
// ambiguous failure, actual invocation and physical cleanup independently.
func Start(ctx context.Context) error {
	_, err := control(ctx, "--system", "--no-ask-password", "--no-pager", "start", "ardents-endpoint.service")
	return err
}

// Only these fixed operations call control. No public input selects arguments,
// executable, manager, environment or deadline; Run joins the actual subprocess.
func control(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		return "", ErrInput
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/systemctl", args...)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := errors.Join(runOriginalManagerCommand(command), bounded.Err(), ctx.Err()); err != nil {
		return "", err
	}
	if diagnostic.body.Len() != 0 {
		return "", ErrUnavailable
	}
	return output.body.String(), nil
}

// The kernel parent-death signal follows the creating thread, not merely its
// process. Keep that thread alive until Run has joined the original child.
// Fatal caller death cannot leave this helper to submit a later manager effect;
// a request already accepted by the manager still requires separate observation.
func runOriginalManagerCommand(command *exec.Cmd) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return command.Run()
}

// Version reads the manager version as a canonical typed observation. Native
// profile admission remains the caller's responsibility.
func Version(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", ErrInput
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/busctl", "--system", "--json=short", "--no-pager", "get-property", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "Version")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := errors.Join(runOriginalManagerCommand(command), bounded.Err(), ctx.Err()); err != nil {
		return "", err
	}
	if diagnostic.body.Len() != 0 {
		return "", ErrUnavailable
	}
	var answer struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}
	body := []byte(strings.TrimSpace(output.body.String()))
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(answer)
	if err != nil || !bytes.Equal(body, canonical) || answer.Type != "s" || answer.Data == "" {
		return "", errors.Join(ErrUnavailable, err)
	}
	return answer.Data, ctx.Err()
}
