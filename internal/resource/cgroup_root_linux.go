//go:build linux

package resource

import (
	"os"
	"path/filepath"
	"strings"
)

func currentCgroupDirectory() (string, error) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	relative, err := cgroupV2ProcessPath(string(raw))
	if err != nil {
		return "", err
	}
	return filepath.Join("/sys/fs/cgroup", filepath.FromSlash(strings.TrimPrefix(relative, "/"))), nil
}
