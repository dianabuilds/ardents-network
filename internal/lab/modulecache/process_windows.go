//go:build windows

package modulecache

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const moduleNewProcessGroup = 0x00000200

func prepareModuleProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: moduleNewProcessGroup}
	return nil
}

func terminateModuleProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "taskkill", "/T", "/F", "/PID", fmt.Sprint(command.Process.Pid)).Run()
	return command.Process.Kill()
}
