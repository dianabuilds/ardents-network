//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type generationDirectoryRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	Device           uint64 `json:"device"`
	Inode            uint64 `json:"inode"`
}

func recordGenerationDirectory(journal, directory string, selected selection) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || identity.Uid != 0 || identity.Gid != 0 || info.Mode().Perm() != 0700 {
		return errors.New("new generation directory identity differs")
	}
	body, err := canonicalJSON(generationDirectoryRecord{Schema: "ardents-endpoint-generation-directory-v1",
		GenerationDigest: selected.GenerationDigest, Device: uint64(identity.Dev), Inode: identity.Ino})
	if err != nil {
		return err
	}
	if err := writeExclusiveGenerationFile(filepath.Join(journal, "generation-directory.json"), body, 0600, 0); err != nil {
		return err
	}
	return syncDirectory(journal)
}

func verifyGenerationDirectory(journal, directory string, selected selection) error {
	body, err := readInstalledFile(filepath.Join(journal, "generation-directory.json"), 4096)
	if err != nil {
		return err
	}
	var record generationDirectoryRecord
	if err := decodeCanonical(body, 4096, &record); err != nil || record.Schema != "ardents-endpoint-generation-directory-v1" || record.GenerationDigest != selected.GenerationDigest {
		return errors.New("generation directory journal differs")
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || identity.Uid != 0 || info.Mode().Perm()&0022 != 0 || uint64(identity.Dev) != record.Device || identity.Ino != record.Inode {
		return errors.New("generation directory no longer matches its owned journal")
	}
	return nil
}
