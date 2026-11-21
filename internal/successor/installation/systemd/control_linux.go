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
