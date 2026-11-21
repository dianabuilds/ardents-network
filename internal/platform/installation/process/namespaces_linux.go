package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// The transitional role invocations opened these intentional kernel links after
// matching their process; that opener retired with the last installed role
// (#604 FJD stage D, ADR-0125). Observe still brackets any retained namespace
// descriptors with the retained proc identity/start clock for the Endpoint and
// Source recovery Invocation lifetimes. Sharing this physical mechanism supplies
// no role policy.
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
