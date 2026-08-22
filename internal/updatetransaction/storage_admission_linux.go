//go:build linux

package updatetransaction

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	ext4SuperMagic        = 0xEF53
	maximumMountinfoBytes = 1 << 20
	maximumMountinfoLines = 4096
	maximumMountinfoLine  = 4096
)

// observeOwnedStorage makes the native, point-in-time Linux observation used
// by Apply after it owns the permanent root lock.  A failure is deliberately
// indistinguishable to the caller from an unsupported activation surface.
func observeOwnedStorage(root string) (resourceObservation, error) {
	if err := validateLinuxOwnedRoot(root); err != nil {
		return resourceObservation{}, err
	}
	mount, err := linuxMountFor(root)
	if err != nil || mount.fileSystem != "ext4" {
		return resourceObservation{}, errors.New("unsupported linux update storage")
	}
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(root, &statfs); err != nil {
		return resourceObservation{}, errors.Join(errCapacityObservation, err)
	}
	if statfs.Type != ext4SuperMagic || statfs.Frsize <= 0 {
		return resourceObservation{}, errors.New("unsupported linux update storage")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return resourceObservation{}, errors.New("linux mount device is unprovable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || mount.device != linuxDevice(stat.Dev) {
		return resourceObservation{}, errors.New("linux mount device is unprovable")
	}
	unit := uint64(statfs.Frsize)
	if uint64(statfs.Bavail) > ^uint64(0)/unit {
		return resourceObservation{}, errCapacityObservation
	}
	return resourceObservation{allocationUnit: unit, availableBytes: uint64(statfs.Bavail) * unit,
		availableItems: uint64(statfs.Ffree), itemsKnown: true}, nil
}

type linuxMount struct {
	id, device, mountPoint, fileSystem string
}

func linuxMountFor(path string) (linuxMount, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return linuxMount{}, err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, maximumMountinfoBytes+1))
	var mounts []linuxMount
	var consumed int
	for lineNumber := 0; ; lineNumber++ {
		line, readErr := reader.ReadString('\n')
		consumed += len(line)
		if consumed > maximumMountinfoBytes || lineNumber >= maximumMountinfoLines || len(line) > maximumMountinfoLine {
			return linuxMount{}, errors.New("mountinfo bounds exceeded")
		}
		if len(line) != 0 {
			mount, parseErr := parseLinuxMountinfo(strings.TrimSuffix(line, "\n"))
			if parseErr != nil {
				return linuxMount{}, parseErr
			}
			for _, previous := range mounts {
				if previous.id == mount.id {
					return linuxMount{}, errors.New("duplicate mount id")
				}
			}
			mounts = append(mounts, mount)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return linuxMount{}, readErr
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return linuxMount{}, err
	}
	var selected linuxMount
	for _, mount := range mounts {
		if mountPrefix(mount.mountPoint, absolute) && len(mount.mountPoint) > len(selected.mountPoint) {
			selected = mount
		}
	}
	if selected.id == "" {
		return linuxMount{}, errors.New("mount not found")
	}
	return selected, nil
}

func parseLinuxMountinfo(line string) (linuxMount, error) {
	fields := strings.Fields(line)
	separator := -1
	for index, field := range fields {
		if field == "-" {
			separator = index
			break
		}
	}
	if separator < 6 || separator+1 >= len(fields) {
		return linuxMount{}, errors.New("malformed mountinfo")
	}
	mountPoint, err := decodeMountinfoPath(fields[4])
	if err != nil || !filepath.IsAbs(mountPoint) || fields[separator+1] == "" {
		return linuxMount{}, errors.New("malformed mountinfo path")
	}
	if _, err := strconv.ParseUint(fields[0], 10, 64); err != nil || !strings.Contains(fields[2], ":") {
		return linuxMount{}, errors.New("malformed mountinfo identity")
	}
	return linuxMount{id: fields[0], device: fields[2], mountPoint: filepath.Clean(mountPoint), fileSystem: fields[separator+1]}, nil
}

func decodeMountinfoPath(value string) (string, error) {
	for _, escape := range []string{"\\040", "\\011", "\\012", "\\134"} {
		value = strings.ReplaceAll(value, escape, map[string]string{"\\040": " ", "\\011": "\t", "\\012": "\n", "\\134": "\\"}[escape])
	}
	if strings.Contains(value, "\\") {
		return "", errors.New("malformed mountinfo escape")
	}
	return value, nil
}

func mountPrefix(mountPoint, path string) bool {
	return mountPoint == "/" || path == mountPoint || strings.HasPrefix(path, mountPoint+"/")
}

func linuxDevice(value uint64) string {
	major := (value >> 8) & 0xfff
	major |= (value >> 32) & ^uint64(0xfff)
	minor := value & 0xff
	minor |= (value >> 12) & ^uint64(0xff)
	return strconv.FormatUint(major, 10) + ":" + strconv.FormatUint(minor, 10)
}

func validateLinuxOwnedRoot(root string) error {
	entries := 0
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entries >= 4096 {
			return errors.New("owned storage walk is invalid")
		}
		entries++
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("owned storage contains an indirect entry")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) {
			return errors.New("owned storage owner is unsupported")
		}
		if info.IsDir() {
			if info.Mode().Perm() != 0o700 {
				return errors.New("owned directory permissions are unsupported")
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || stat.Nlink != 1 {
			return errors.New("owned file permissions are unsupported")
		}
		return nil
	})
}
