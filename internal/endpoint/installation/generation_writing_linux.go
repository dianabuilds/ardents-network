//go:build linux

package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// writeGeneration requires an already provisioned root-controlled parent.
// A partial candidate is retained; selection and fixed resources are untouched.
func writeGeneration(ctx context.Context, root string, authorization Authorization, request Request, uid, gid uint32, roots []rootBinding) (result selection, returnedErr error) {
	if os.Geteuid() != 0 || ctx == nil {
		return selection{}, errors.New("generation writer requires root and context")
	}
	if root != request.InstallationRoot {
		return selection{}, errors.New("generation writer installation root differs")
	}
	files, selected, err := assembleGeneration(request, authorization, uid, gid, roots)
	if err != nil {
		return selection{}, err
	}
	parent := filepath.Join(root, "generations")
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil {
			return selection{}, err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !ok || owner.Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return selection{}, errors.New("generation writer ancestor is untrusted")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return selection{}, err
	}
	lease, err := acquireInstallationLease(root)
	if err != nil {
		return selection{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, lease.Close()) }()
	directory := filepath.Join(parent, selected.GenerationDigest)
	journal, err := beginGenerationJournal(root, selected)
	if err != nil {
		return selection{}, err
	}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, appendGenerationRecord(journal, "0002.json", selected, "generation-write-failed", returnedErr))
		}
	}()
	if err := os.Mkdir(directory, 0700); err != nil {
		return selection{}, fmt.Errorf("create exclusive generation: %w", err)
	}
	if err := syncDirectory(parent); err != nil {
		return selection{}, err
	}
	if err := recordGenerationDirectory(journal, directory, selected); err != nil {
		return selection{}, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return selection{}, err
		}
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := writeExclusiveGenerationFile(filepath.Join(directory, name), files[name], mode, gid); err != nil {
			return selection{}, fmt.Errorf("write generation %s: %w", name, err)
		}
	}
	if err := os.Chown(directory, 0, int(gid)); err != nil {
		return selection{}, err
	}
	if err := os.Chmod(directory, 0750); err != nil {
		return selection{}, err
	}
	if err := syncDirectory(directory); err != nil {
		return selection{}, err
	}
	if err := syncDirectory(parent); err != nil {
		return selection{}, err
	}
	if err := ctx.Err(); err != nil {
		return selection{}, err
	}
	if err := appendGenerationRecord(journal, "0002.json", selected, "generation-staged", nil); err != nil {
		return selection{}, err
	}
	return selected, nil
}

func writeExclusiveGenerationFile(path string, body []byte, mode os.FileMode, gid uint32) error {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	_, writeErr := file.Write(body)
	if writeErr == nil {
		writeErr = file.Chown(0, int(gid))
	}
	if writeErr == nil {
		writeErr = file.Chmod(mode)
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	return errors.Join(writeErr, file.Close())
}

func syncDirectory(path string) error {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	directory := os.NewFile(uintptr(fd), path)
	return errors.Join(directory.Sync(), directory.Close())
}
