//go:build linux

package endpoint

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func createTextRoleObservationOutput(root, carrier string) (string, error) {
	worktree, err := textRoleObservationWorktree()
	if err != nil {
		return "", err
	}
	return createTextRoleObservationOutputAgainstWorktree(root, carrier, worktree)
}

func createTextRoleObservationOutputAgainstWorktree(root, carrier, worktree string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", errors.New("capture root must be a clean absolute path")
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	defer unix.Close(fd)
	canonicalRoot, err := filepath.EvalSymlinks(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil {
		return "", err
	}
	canonicalWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return "", err
	}
	if durablePathContains(canonicalWorktree, canonicalRoot) {
		return "", errors.New("capture root must be outside the Git worktree")
	}
	for attempt := 0; attempt < 32; attempt++ {
		var suffix [16]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", err
		}
		name := carrier + "-" + hex.EncodeToString(suffix[:])
		if err := unix.Mkdirat(fd, name, 0o700); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return "", err
		}
		child, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return "", err
		}
		output, resolveErr := filepath.EvalSymlinks(fmt.Sprintf("/proc/self/fd/%d", child))
		closeErr := unix.Close(child)
		if resolveErr != nil || closeErr != nil {
			return "", errors.Join(resolveErr, closeErr)
		}
		if durablePathContains(canonicalWorktree, output) {
			return "", errors.New("capture output is inside the Git worktree")
		}
		return output, nil
	}
	return "", errors.New("could not allocate unique role observation directory")
}

func TestCreateTextRoleObservationOutputRejectsCaptureInsideWorktree(t *testing.T) {
	worktree := t.TempDir()
	capture := filepath.Join(worktree, "captures")
	if err := os.Mkdir(capture, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "worktree-link")
	if err := os.Symlink(worktree, link); err != nil {
		t.Fatal(err)
	}
	if _, err := createTextRoleObservationOutputAgainstWorktree(filepath.Join(link, "captures"), "carrier", worktree); err == nil || !strings.Contains(err.Error(), "outside the Git worktree") {
		t.Fatalf("capture output error = %v, want worktree rejection", err)
	}
}

func copyTextRoleDurableRoot(root textRoleDurableRoot, output string) (textRoleDurableRootCapture, error) {
	fd, err := unix.Open(root.path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return textRoleDurableRootCapture{}, err
	}
	defer unix.Close(fd)
	capture := textRoleDurableRootCapture{Name: root.name}
	destination := filepath.Join(output, root.name)
	if err := os.Mkdir(destination, 0o700); err != nil {
		return textRoleDurableRootCapture{}, err
	}
	if err := copyTextRoleDurableDirectory(fd, destination, "", &capture); err != nil {
		return textRoleDurableRootCapture{}, err
	}
	return capture, nil
}

func copyTextRoleDurableDirectory(fd int, destination, relative string, capture *textRoleDurableRootCapture) error {
	readFD, err := unix.Dup(fd)
	if err != nil {
		return err
	}
	directory := os.NewFile(uintptr(readFD), "durable-root")
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("durable root contains symlink")
		}
		name := entry.Name()
		childRelative := filepath.Join(relative, name)
		if entry.IsDir() {
			child, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				return err
			}
			childDestination := filepath.Join(destination, name)
			if err := os.Mkdir(childDestination, 0o700); err != nil {
				unix.Close(child)
				return err
			}
			err = copyTextRoleDurableDirectory(child, childDestination, childRelative, capture)
			closeErr := unix.Close(child)
			if err != nil || closeErr != nil {
				return errors.Join(err, closeErr)
			}
			continue
		}
		copied, err := copyTextRoleDurableFileAt(fd, name, filepath.Join(destination, name), childRelative)
		if err != nil {
			return err
		}
		capture.Files = append(capture.Files, copied)
	}
	return nil
}

func copyTextRoleDurableFileAt(directory int, name, destination, relative string) (textRoleDurableFileCapture, error) {
	fd, err := unix.Openat(directory, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return textRoleDurableFileCapture{}, err
	}
	from := os.NewFile(uintptr(fd), "durable-file")
	defer from.Close()
	var status unix.Stat_t
	if err := unix.Fstat(fd, &status); err != nil {
		return textRoleDurableFileCapture{}, err
	}
	if status.Mode&unix.S_IFMT != unix.S_IFREG {
		return textRoleDurableFileCapture{}, fmt.Errorf("durable root contains non-regular file %q", name)
	}
	to, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return textRoleDurableFileCapture{}, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(to, hash), from)
	syncErr, closeErr := to.Sync(), to.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		return textRoleDurableFileCapture{}, errors.Join(copyErr, syncErr, closeErr)
	}
	return textRoleDurableFileCapture{Path: filepath.ToSlash(relative), SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: size}, nil
}
