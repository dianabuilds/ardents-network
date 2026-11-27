package custody

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Linux uses the same superblock magic for ext2, ext3 and ext4.
const (
	extFilesystem   = 0xef53
	tmpfsFilesystem = 0x01021994
	procFilesystem  = 0x9fa0
)

func checkPrivatePermissionDestination(path string) error {
	destination, err := filepath.Abs(path)
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	parent, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	info, statErr := parent.Stat()
	if statErr != nil || !info.IsDir() {
		return errors.Join(ErrPrivatePermissionUnavailable, statErr, parent.Close())
	}
	return errors.Join(checkPrivatePermissionFilesystem(parent), parent.Close())
}

func checkPrivatePermissionFilesystem(file *os.File) error {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	if fs.Type == tmpfsFilesystem {
		return nil
	}
	if fs.Type != extFilesystem {
		return ErrPrivatePermissionUnavailable
	}
	// The descriptor pins its mount. Match that mount's ID, not the pathname,
	// against the kernel's mount table; an unmounted or hidden entry refuses.
	var stat unix.Statx_t
	if err := unix.Statx(int(file.Fd()), "", unix.AT_EMPTY_PATH, unix.STATX_MNT_ID, &stat); err != nil || stat.Mask&unix.STATX_MNT_ID == 0 {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	mounts, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	defer mounts.Close()
	var proc syscall.Statfs_t
	if err := syscall.Fstatfs(int(mounts.Fd()), &proc); err != nil || proc.Type != procFilesystem {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	if err := requireExt4Mount(mounts, stat.Mnt_id); err != nil {
		return errors.Join(ErrPrivatePermissionUnavailable, err)
	}
	return nil
}

// requireExt4Mount reads the kernel mountinfo grammar and accepts exactly one
// entry for the retained mount ID. Incomplete or ambiguous input fails closed.
func requireExt4Mount(mounts io.Reader, mountID uint64) error {
	scanner := bufio.NewScanner(mounts)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	found := false
	for scanner.Scan() {
		left, right, ok := strings.Cut(scanner.Text(), " - ")
		if !ok {
			return errors.New("invalid kernel mount information")
		}
		fields, filesystem := strings.Fields(left), strings.Fields(right)
		if len(fields) < 6 || len(filesystem) < 3 {
			return errors.New("invalid kernel mount information")
		}
		id, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid kernel mount ID: %w", err)
		}
		if id == mountID {
			if found || filesystem[0] != "ext4" {
				return errors.New("original mount is not uniquely ext4")
			}
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !found {
		return errors.New("original mount is not visible")
	}
	return nil
}

func privatePermissionFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
