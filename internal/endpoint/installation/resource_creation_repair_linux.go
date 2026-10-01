//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func repairCreatedInstallationFile(ctx context.Context, path string, expected []byte, mode os.FileMode, gid uint32, journal string, selected selection) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return errors.New("initial resource repair requires root and context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateCreatedInstallationFile(path, expected, mode, gid, journal, selected); err != nil {
		return err
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	recordPath := filepath.Join(journal, "creations", digestHex([]byte(path))+".json")
	if err := requirePrivateJournalFile(recordPath); err != nil {
		return err
	}
	body, err := readInstalledFile(recordPath, 4096)
	if err != nil {
		return err
	}
	var record replacementRecord
	if err := decodeCanonical(body, 4096, &record); err != nil || record.Schema != "ardents-endpoint-installation-creation-v1" ||
		record.Path != path || record.GenerationDigest != selected.GenerationDigest || record.CandidateDigest != digestHex(expected) ||
		record.PreviousDigest != "" || record.Mode != uint32(mode) || record.GID != gid {
		return errors.New("repair-required: open initial repair birth binding differs")
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || identity.Uid != 0 || identity.Nlink != 1 ||
		uint64(identity.Dev) != record.Device || identity.Ino != record.Inode ||
		(identity.Gid != 0 && identity.Gid != gid) || (info.Mode().Perm() != 0600 && info.Mode().Perm() != mode) {
		return errors.New("repair-required: open initial repair inode differs from birth record")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := repairOpenGenerationFile(file, info, expected, mode, gid); err != nil {
		return err
	}
	return errors.Join(syncDirectory(filepath.Dir(path)), ctx.Err())
}
