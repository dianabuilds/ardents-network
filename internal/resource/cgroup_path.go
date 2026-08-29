package resource

import (
	"errors"
	"path"
	"strings"
)

func cgroupV2ProcessPath(raw string) (string, error) {
	result := ""
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 3 || fields[0] != "0" || fields[1] != "" {
			continue
		}
		candidate := fields[2]
		if result != "" || !strings.HasPrefix(candidate, "/") || strings.Contains(candidate, "\x00") ||
			path.Clean(candidate) != candidate {
			return "", errors.New("process cgroup v2 path is invalid")
		}
		result = candidate
	}
	if result == "" {
		return "", errors.New("process cgroup v2 path is unavailable")
	}
	return result, nil
}
