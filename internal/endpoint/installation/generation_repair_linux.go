//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

func repairOwnedGeneration(ctx context.Context, root string, intent transitionIntent, files map[string][]byte) error {
	directory := filepath.Join(root, "generations", intent.Candidate.GenerationDigest)
	journal := filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	if err := verifyGenerationDirectory(journal, directory, intent.Candidate); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > len(files) {
		return errors.New("repair-required: generation inventory is unavailable")
	}
	for _, entry := range entries {
		if _, known := files[entry.Name()]; !known {
			return errors.New("repair-required: generation contains foreign entries")
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := repairGenerationFile(filepath.Join(directory, name), files[name], mode, intent.CandidateBinding.GID); err != nil {
			return err
		}
	}
	if err := os.Chown(directory, 0, int(intent.CandidateBinding.GID)); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0750); err != nil {
		return err
	}
	if err := errors.Join(syncDirectory(directory), syncDirectory(filepath.Dir(directory))); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return recordSuccessorPhase(journal, "recovery-staged.json", intent.Candidate, "fresh-authorized-generation-repaired")
}

func repairGenerationFile(path string, expected []byte, mode os.FileMode, gid uint32) (returnedErr error) {
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return writeExclusiveGenerationFile(path, expected, mode, gid)
	}
	if err != nil {
		return err
	}
	identity, ok := before.Sys().(*syscall.Stat_t)
	if !ok || !before.Mode().IsRegular() || identity.Uid != 0 || identity.Nlink != 1 ||
		(identity.Gid != 0 && identity.Gid != gid) || (before.Mode().Perm() != 0600 && before.Mode().Perm() != mode) {
		return errors.New("repair-required: incomplete generation file is foreign")
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return errors.New("repair-required: incomplete generation inode changed")
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
	if err != nil || len(body) > len(expected) || !bytes.Equal(body, expected[:len(body)]) {
		return errors.New("repair-required: incomplete generation bytes are not its authorized prefix")
	}
	if bytes.Equal(body, expected) && before.Mode().Perm() == mode && identity.Gid == gid {
		return nil
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := file.Write(expected); err != nil {
		return err
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	return file.Sync()
}
