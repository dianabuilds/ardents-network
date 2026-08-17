//go:build windows

package blockedverify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const supplyNewProcessGroup = 0x00000200

func prepareSupplyProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: supplyNewProcessGroup}
}

func terminateSupplyProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "taskkill", "/T", "/F", "/PID", fmt.Sprint(command.Process.Pid)).Run()
	return command.Process.Kill()
}
