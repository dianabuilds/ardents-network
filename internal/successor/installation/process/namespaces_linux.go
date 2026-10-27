package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// These intentional kernel links are opened only after the separate fixed
// opener has matched its process. Observe brackets them with the retained proc
// identity/start clock. Sharing this physical mechanism supplies no role policy.
func (pin *Invocation) retainNamespaces(ctx context.Context) error {
	pin.namespaces = make(map[string]*os.File)
	for _, name := range []string{"mnt", "pid", "cgroup", "uts", "user", "net", "ipc"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join("/proc", strconv.FormatUint(uint64(pin.pid), 10), "ns", name), os.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		pin.namespaces[name] = file
	}
	return pin.observeNamespaces(ctx)
}

func (pin *Invocation) observeNamespaces(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for name, file := range pin.namespaces {
		opened, err := file.Stat()
		named, pathErr := os.Stat(filepath.Join("/proc", strconv.FormatUint(uint64(pin.pid), 10), "ns", name))
		var fs syscall.Statfs_t
		fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
		if err != nil || pathErr != nil || fsErr != nil || fs.Type != 0x6e736673 || !os.SameFile(opened, named) {
			return errors.Join(errBinding, err, pathErr, fsErr)
		}
	}
	return ctx.Err()
}
