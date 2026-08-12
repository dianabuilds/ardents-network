//go:build linux

package resource

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

func checkPlacement(profile profile) error {
	for name, want := range profile.cgroup {
		raw, err := os.ReadFile("/sys/fs/cgroup/" + name)
		if err != nil || strings.TrimSpace(string(raw)) != want {
			return errors.New("resource guard cgroup placement does not match its profile")
		}
	}
	var files syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &files); err != nil || files.Cur != profile.noFile || files.Max != profile.noFile {
		return errors.New("resource guard file limit does not match its profile")
	}
	return nil
}
