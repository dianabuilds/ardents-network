//go:build linux

package endpoint

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const textCgroup2Magic = 0x63677270

// pinTextWorkerCgroup retains the kernel events object of the observed unit.
// A later pathname lookup cannot substitute another invocation's cgroup.
// This is cleanup ownership, not evidence authorizing a Principal or Grant.
func pinTextWorkerCgroup(instance textWorkerInstance) (*os.File, error) {
	if !textWorkerCgroupPath(instance.cgroup, instance.name, instance.role) {
		return nil, errors.New("text worker cgroup identity is invalid")
	}
	path := "/sys/fs/cgroup" + instance.cgroup
	before, err := textInstalledPath(path, true)
	if err != nil {
		return nil, errors.New("text worker cgroup ownership is unavailable")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, errors.New("text worker cgroup could not be pinned")
	}
	directory := os.NewFile(uintptr(fd), path)
	defer directory.Close()
	after, err := directory.Stat()
	var filesystem syscall.Statfs_t
	if err != nil || !os.SameFile(before, after) || syscall.Fstatfs(fd, &filesystem) != nil || filesystem.Type != textCgroup2Magic {
		return nil, errors.New("text worker cgroup filesystem is unverified")
	}
	eventFD, err := syscall.Openat(fd, "cgroup.events", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, errors.New("text worker cgroup events are unavailable")
	}
	events := os.NewFile(uintptr(eventFD), "text worker cgroup.events")
	var eventStat syscall.Stat_t
	if syscall.Fstat(eventFD, &eventStat) != nil || eventStat.Mode&syscall.S_IFMT != syscall.S_IFREG || eventStat.Uid != 0 || eventStat.Mode&0022 != 0 {
		_ = events.Close()
		return nil, errors.New("text worker cgroup events ownership is unverified")
	}
	removed, _, err := readTextWorkerCgroup(events)
	if err != nil || removed {
		_ = events.Close()
		return nil, errors.New("text worker cgroup disappeared before pinning")
	}
	return events, nil
}

func textWorkerCgroupPath(group, name, role string) bool {
	return textWorkerUnit(name, role) && len(group) <= 4096 &&
		strings.HasPrefix(group, "/system.slice/") && strings.HasSuffix(group, "/"+name) &&
		filepath.Clean(group) == group && !strings.ContainsAny(group, "\x00\r\n")
}

// readTextWorkerCgroup accepts ENODEV only on the already-pinned cgroup v2
// events object. kernfs deactivates this core file when its group is removed;
// unlike inode link counts, this survives path reuse without observing a new
// group. Seek also takes a kernfs active reference and can report ENODEV.
// Removal requires no live processes or child cgroups. Other seek/read errors
// and missing/unknown populated observations never mean successful cleanup.
func readTextWorkerCgroup(events *os.File) (removed, populated bool, resultErr error) {
	if events == nil {
		return false, false, errors.New("text worker cgroup pin is absent")
	}
	if _, err := events.Seek(0, io.SeekStart); err != nil {
		if errors.Is(err, syscall.ENODEV) {
			return true, false, nil
		}
		return false, false, errors.New("text worker cgroup observation could not rewind")
	}
	body, err := io.ReadAll(io.LimitReader(events, 1025))
	if errors.Is(err, syscall.ENODEV) && len(body) == 0 {
		return true, false, nil
	}
	if err != nil || len(body) > 1024 {
		return false, false, errors.New("text worker cgroup observation is unavailable")
	}
	populated, err = decodeTextWorkerCgroup(body)
	return false, populated, err
}

func decodeTextWorkerCgroup(body []byte) (bool, error) {
	fields := strings.Fields(string(body))
	if len(body) > 1024 || len(fields) != 4 {
		return false, errors.New("text worker cgroup observation is invalid")
	}
	values := make(map[string]string, 2)
	for index := 0; index < len(fields); index += 2 {
		key, value := fields[index], fields[index+1]
		if (key != "populated" && key != "frozen") || (value != "0" && value != "1") || values[key] != "" {
			return false, errors.New("text worker cgroup observation is unknown")
		}
		values[key] = value
	}
	return values["populated"] == "1", nil
}
