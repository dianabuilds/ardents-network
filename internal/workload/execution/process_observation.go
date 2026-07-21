package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
)

// ProcessMatchesConfig verifies that the live process still matches the stored
// workload command line before runtime recovery trusts it as the workload owner.
func ProcessMatchesConfig(pid int, raw string) bool {
	if pid <= 0 || !ProcessRunning(pid) {
		return false
	}
	cfg, err := parseProcessSpec(raw)
	if err != nil {
		return false
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	cmdline, err := proc.CmdlineSlice()
	if err != nil {
		return false
	}
	return commandMatches(cmdline, cfg.Command, cfg.Args)
}

func commandMatches(cmdline []string, command string, args []string) bool {
	if !sameExecutable(cmdline[0], command) {
		return false
	}
	actualArgs := cmdline[1:]
	if argsMatch(actualArgs, args) {
		return true
	}
	return argsMatch(actualArgs, flattenArgs(args))
}

func sameExecutable(actual, expected string) bool {
	actual = cleanToken(actual)
	expected = cleanToken(expected)
	if actual == "" || expected == "" {
		return false
	}
	if runtime.GOOS == "windows" {
		actual = strings.ToLower(actual)
		expected = strings.ToLower(expected)
	}
	return actual == expected || filepath.Base(actual) == filepath.Base(expected)
}

func sameArg(actual, expected string) bool {
	actual = cleanToken(actual)
	expected = cleanToken(expected)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(actual, expected)
	}
	return actual == expected
}

func cleanToken(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"")
}

func argsMatch(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if !sameArg(actual[i], expected[i]) {
			return false
		}
	}
	return true
}

func flattenArgs(args []string) []string {
	flat := make([]string, 0, len(args))
	for _, arg := range args {
		fields := strings.Fields(arg)
		if len(fields) <= 1 {
			flat = append(flat, arg)
			continue
		}
		flat = append(flat, fields...)
	}
	return flat
}

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
