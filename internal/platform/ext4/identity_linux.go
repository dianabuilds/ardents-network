package ext4

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Linux uses the same superblock magic for ext2, ext3 and ext4.
const (
	extFilesystem  = 0xef53
	procFilesystem = 0x9fa0
)

// Require verifies the filesystem of file's retained mount. An unavailable or
// ambiguous kernel observation is a refusal; the caller keeps file open.
func Require(file *os.File) error {
	if file == nil {
		return errors.New("original filesystem descriptor is unavailable")
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil {
		return err
	}
	if fs.Type != extFilesystem {
		return errors.New("original filesystem is not ext4")
	}
	// The descriptor pins its mount. Match that mount's ID, not its pathname,
	// against the kernel's mount table; an unmounted or hidden entry refuses.
	var stat unix.Statx_t
	if err := unix.Statx(int(file.Fd()), "", unix.AT_EMPTY_PATH, unix.STATX_MNT_ID, &stat); err != nil {
		return err
	}
	if stat.Mask&unix.STATX_MNT_ID == 0 {
		return errors.New("original mount identity is unavailable")
	}
	mounts, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	defer mounts.Close()
	var proc syscall.Statfs_t
	if err := syscall.Fstatfs(int(mounts.Fd()), &proc); err != nil {
		return err
	}
	if proc.Type != procFilesystem {
		return errors.New("kernel mount information is unavailable")
	}
	return requireMount(mounts, stat.Mnt_id)
}

// requireMount accepts exactly one ext4 entry for the retained mount ID.
// Incomplete or ambiguous input fails closed.
func requireMount(mounts io.Reader, mountID uint64) error {
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
