//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// createInstallationFile records the exclusively created inode before writing
// its intended bytes. A crash before the record is durable requires repair;
// absence preflight alone never authorizes adoption of an existing file.
func createInstallationFile(ctx context.Context, path string, body []byte, mode os.FileMode, gid uint32, journal string, selected selection) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || len(body) == 0 || len(body) > 64<<20 {
		return errors.New("initial resource creation input is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkRootAncestors(filepath.Dir(path), false); err != nil {
		return err
	}
	records := filepath.Join(journal, "creations")
	if _, err := os.Lstat(records); os.IsNotExist(err) {
		if err := createRootDirectory(records, 0700, 0); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := requirePrivateJournalDirectory(records); err != nil {
		return err
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != 0 || identity.Gid != 0 || identity.Nlink != 1 || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return errors.New("initial resource birth identity differs")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	record := replacementRecord{Schema: "ardents-endpoint-installation-creation-v1", GenerationDigest: selected.GenerationDigest,
		Path: path, Device: uint64(identity.Dev), Inode: identity.Ino, Mode: uint32(mode), GID: gid, CandidateDigest: digestHex(body)}
	recordBytes, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	if err := writeExclusiveGenerationFile(filepath.Join(records, digestHex([]byte(path))+".json"), recordBytes, 0600, 0); err != nil {
		return err
	}
	if err := syncDirectory(records); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return errors.Join(syncDirectory(filepath.Dir(path)), ctx.Err())
}
