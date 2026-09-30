//go:build linux

package installation

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func observeBinding(checked checkedBinding) error {
	if err := observeAccountAndRoots(checked, true); err != nil {
		return err
	}
	return observeFixedResources(checked)
}

func observeAccountAndRoots(checked checkedBinding, selected bool) error {
	account, err := user.Lookup("ardents-endpoint")
	if err != nil || account.Uid != strconv.FormatUint(uint64(checked.binding.UID), 10) || account.Gid != strconv.FormatUint(uint64(checked.binding.GID), 10) {
		return errors.New("installation Endpoint account identity differs")
	}
	group, err := user.LookupGroup("ardents-endpoint")
	if err != nil || group.Gid != account.Gid {
		return errors.New("installation Endpoint group identity differs")
	}
	if err := checkGenerationAccess(checked.binding.InstallationRoot, checked, selected); err != nil {
		return err
	}
	for _, root := range checked.binding.MutableRoots {
		info, err := os.Lstat(root.Path)
		if err != nil || !info.IsDir() {
			return errors.New("installation mutable root is unavailable")
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(identity.Dev) != root.Device || identity.Ino != root.Inode || identity.Uid != checked.binding.UID || identity.Gid != checked.binding.GID || info.Mode().Perm() != 0700 {
			return errors.New("installation mutable root identity or access differs")
		}
		for ancestor := filepath.Dir(root.Path); ; ancestor = filepath.Dir(ancestor) {
			parent, err := os.Lstat(ancestor)
			if err != nil || !parent.IsDir() {
				return errors.New("installation mutable root ancestor is unavailable")
			}
			owner, ok := parent.Sys().(*syscall.Stat_t)
			if !ok || owner.Uid != 0 && owner.Uid != checked.binding.UID || parent.Mode().Perm()&0022 != 0 {
				return errors.New("installation mutable root ancestor is untrusted")
			}
			if filepath.Dir(ancestor) == ancestor {
				break
			}
		}
	}
	return nil
}

func observeFixedResources(checked checkedBinding) error {
	artifact, err := worker.LoadArtifact(worker.Text)
	if err != nil {
		return err
	}
	installedDigests := artifact.FileDigests()
	for path, name := range fixedResources() {
		body, err := worker.ReadInstalledFile(path, 64<<20)
		if err != nil {
			return err
		}
		expected := checked.files[name]
		if name == "ardents-endpoint.service" {
			expected = checked.files["endpoint-unit.service"]
		}
		if digestHex(body) != digestHex(expected) {
			return errors.New("installation fixed resources differ from the selected generation")
		}
		if pinned, present := installedDigests[path]; present && digestHex(body) != hex.EncodeToString(pinned[:]) {
			return errors.New("installation worker reference differs from its fixed bytes")
		}
	}
	return nil
}

func readInstalledFile(path string, maximum int64) ([]byte, error) {
	return worker.ReadInstalledFile(path, maximum)
}

func observePlatform(ctx context.Context) error {
	if runtime.GOARCH != "amd64" {
		return errors.New("protected installation requires Ubuntu24.04 amd64")
	}
	body, err := worker.ReadInstalledFile("/usr/lib/os-release", 16<<10)
	if err != nil {
		return err
	}
	fields := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			if _, duplicate := fields[key]; duplicate {
				return errors.New("installation OS identity contains duplicate fields")
			}
			fields[key] = strings.Trim(value, "\"")
		}
	}
	if fields["ID"] != "ubuntu" || fields["VERSION_ID"] != "24.04" {
		return errors.New("protected installation requires Ubuntu24.04")
	}
	version, err := worker.ManagerVersion(ctx)
	if err != nil || version != 255 {
		return errors.Join(errors.New("protected installation requires actual system manager255"), err)
	}
	var filesystem syscall.Statfs_t
	if syscall.Statfs("/sys/fs/cgroup", &filesystem) != nil || filesystem.Type != 0x63677270 {
		return errors.New("protected installation requires cgroup v2")
	}
	return nil
}

func checkGenerationAccess(root string, checked checkedBinding, selected bool) error {
	for _, directory := range []string{root, filepath.Join(root, "generations"), checked.directory} {
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() {
			return errors.New("installation generation directory is unavailable")
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || identity.Uid != 0 || identity.Gid != checked.binding.GID || info.Mode().Perm()&0022 != 0 {
			return errors.New("installation generation directory ownership differs")
		}
	}
	paths := []string{filepath.Join(checked.directory, "binding.json")}
	if selected {
		paths = append(paths, filepath.Join(root, "selection.json"))
	}
	for name := range checked.files {
		paths = append(paths, filepath.Join(checked.directory, name))
	}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		mode := os.FileMode(0640)
		if filepath.Base(path) == "ardents-linux-amd64" || filepath.Base(path) == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if !ok || identity.Uid != 0 || identity.Gid != checked.binding.GID || info.Mode().Perm() != mode {
			return errors.New("installation generation file access differs")
		}
	}
	entries, err := os.ReadDir(checked.directory)
	if err != nil || len(entries) != len(checked.files)+1 {
		return errors.New("installation generation has an unexpected file inventory")
	}
	for _, entry := range entries {
		if entry.Name() != "binding.json" && checked.files[entry.Name()] == nil {
			return errors.New("installation generation contains an unknown file")
		}
	}
	return nil
}

func fixedResources() map[string]string {
	return map[string]string{
		"/usr/lib/ardents/text-worker-root/ardents-text":      "ardents-text-linux-amd64",
		"/etc/systemd/system/ardents-text-reader@.service":    "ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service": "ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket":      "ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket":   "ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules":   "50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf":               "ardents-text.conf",
		"/etc/systemd/system/ardents-endpoint.service":        "ardents-endpoint.service",
	}
}
