//go:build linux && endpoint_launch_native

package endpoint

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// RetainLaunchScope retains the independent kernel descriptor before Stop. Path disappearance
// alone cannot report that the original cgroup joined.
func RetainLaunchScope(t testing.TB, directory string) *os.File {
	t.Helper()
	file, err := os.Open(filepath.Join(directory, "cgroup.events"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0x63677270 {
		t.Fatal("original scope is not cgroup v2:", err)
	}
	if populated, err := LaunchScopePopulated(file); err != nil || !populated {
		t.Fatalf("expected populated original scope: populated=%v err=%v", populated, err)
	}
	return file
}

// LaunchScopePopulated reads the retained cgroup v2 descriptor.
func LaunchScopePopulated(file *os.File) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		if errors.Is(err, syscall.ENODEV) {
			return false, nil
		}
		return false, err
	}
	body, err := io.ReadAll(io.LimitReader(file, 1025))
	if errors.Is(err, syscall.ENODEV) && len(body) == 0 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	switch {
	case len(body) > 1024:
		return false, errors.New("original cgroup.events exceeds observation bound")
	case bytes.HasPrefix(body, []byte("populated 0\n")):
		return false, nil
	case bytes.HasPrefix(body, []byte("populated 1\n")):
		return true, nil
	default:
		return false, fmt.Errorf("unrecognized original cgroup.events: %q", body)
	}
}
