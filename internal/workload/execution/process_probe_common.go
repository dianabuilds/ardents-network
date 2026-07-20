package execution

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func StopProcessByPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil && !alreadyFinished(err) {
		return err
	}
	return waitForPIDStop(pid, time.Now().Add(2*time.Second))
}

func waitForPIDStop(pid int, deadline time.Time) error {
	for time.Now().Before(deadline) {
		if !ProcessRunning(pid) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("process %d is still running after stop deadline", pid)
}

func alreadyFinished(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already finished")
}
