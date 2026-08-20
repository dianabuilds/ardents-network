//go:build ignore

package r050

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const ext4SuperMagic = 0xef53

func preparePlatformRoot(root string) error {
	return os.Chmod(root, 0o700)
}

func validatePlatformRoot(root string) error {
	if err := rejectLinuxLinks(root); err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("root mode %s: %w", info.Mode(), errActivationUnsupported)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("root owner mismatch: %w", errActivationUnsupported)
	}
	var filesystem unix.Statfs_t
	if err := unix.Statfs(root, &filesystem); err != nil {
		return fmt.Errorf("statfs root: %w", err)
	}
	if uint64(filesystem.Type) != ext4SuperMagic {
		return fmt.Errorf("filesystem type %#x: %w", filesystem.Type, errActivationUnsupported)
	}
	return nil
}

func platformSecureTemporary(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure activation temp: %w", err)
	}
	return nil
}

func validatePlatformTemporary(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat activation temp: %w", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("unsafe activation temp mode=%s: %w", info.Mode(), errActivationUnsupported)
	}
	return nil
}

func platformReplace(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return fmt.Errorf("cross-mount activation: %w", errActivationUnsupported)
		}
		return fmt.Errorf("rename activation: %w", err)
	}
	return nil
}

func platformSyncParent(root string) error {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open activation directory: %w", err)
	}
	defer unix.Close(fd)
	if err := unix.Fsync(fd); err != nil {
		return fmt.Errorf("sync activation directory: %w", err)
	}
	return nil
}

func validatePlatformCommitted(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("unsafe committed activation: %w", errActivationUnsupported)
	}
	return nil
}

func platformReadActivation(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func platformManifest(root string) (string, error) {
	var filesystem unix.Statfs_t
	if err := unix.Statfs(root, &filesystem); err != nil {
		return "", err
	}
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return "", err
	}
	return fmt.Sprintf("kernel=%s filesystem_magic=%#x", unix.ByteSliceToString(uname.Release[:]), filesystem.Type), nil
}

func rejectLinuxLinks(path string) error {
	clean := filepath.Clean(path)
	current := string(filepath.Separator)
	volume := filepath.VolumeName(clean)
	rest := clean[len(volume):]
	if volume != "" {
		current = volume + string(filepath.Separator)
	}
	for _, component := range splitPathComponents(rest) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("lstat %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink component %s: %w", current, errActivationUnsupported)
		}
	}
	return nil
}

func splitPathComponents(path string) []string {
	var components []string
	for path != "" && path != string(filepath.Separator) && path != "." {
		dir, base := filepath.Split(path)
		if base != "" {
			components = append([]string{base}, components...)
		}
		path = filepath.Clean(dir)
	}
	return components
}

func platformHoldActivation(root string) (func(), bool, error) {
	file, err := os.Open(filepath.Join(root, activationFile))
	if err != nil {
		return nil, false, err
	}
	return func() { _ = file.Close() }, false, nil
}

func platformCreateLinkedRoot(target, link string) error {
	return os.Symlink(target, link)
}

func makePlatformRootUnsafe(root string) error {
	return os.Chmod(root, 0o755)
}
