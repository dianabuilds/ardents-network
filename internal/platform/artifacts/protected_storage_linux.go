//go:build linux

package artifacts

import (
	"errors"
	"os"
	"syscall"
)

// CheckProtected adds the Linux ext4 storage floor to the shared
// original-descriptor checks for offline root preparation/signing. The ext4
// magic is a genuine Linux storage policy; the macOS floor is a separate
// product decision, so this check keeps its Linux boundary (ADR-0129).
// Runtime/client input custody keeps its separately admitted caller policy.
func (in *Inputs) CheckProtected() error {
	if os.Geteuid() != 0 {
		return ErrInput
	}
	for _, input := range in.files {
		info, err := input.file.Stat()
		if err != nil || info.Sys().(*syscall.Stat_t).Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.Join(ErrInput, err)
		}
		if info.IsDir() {
			continue
		}
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(input.file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return errors.Join(ErrInput, err)
		}
	}
	return in.Check()
}
