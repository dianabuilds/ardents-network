//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type transitionRecord struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
	Phase            string `json:"phase"`
	OriginalError    string `json:"original_error,omitempty"`
}

func beginGenerationJournal(root string, selected selection) (string, error) {
	parent := filepath.Join(root, "journals")
	if err := os.Mkdir(parent, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err := os.Lstat(parent)
	if err != nil {
		return "", err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || owner.Uid != 0 || owner.Gid != 0 || info.Mode().Perm() != 0700 {
		return "", errors.New("installation journal directory is untrusted")
	}
	if err := syncDirectory(root); err != nil {
		return "", err
	}
	directory := filepath.Join(parent, selected.GenerationDigest)
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", err
	}
	if err := syncDirectory(parent); err != nil {
		return "", err
	}
	if err := appendGenerationRecord(directory, "0001.json", selected, "writing-generation", nil); err != nil {
		return "", err
	}
	return directory, nil
}

func appendGenerationRecord(directory, name string, selected selection, phase string, original error) error {
	record := transitionRecord{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: selected.GenerationDigest, BindingDigest: selected.BindingDigest, Phase: phase}
	if original != nil {
		record.OriginalError = original.Error()
	}
	body, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	if len(body) > 64<<10 {
		return errors.New("installation journal record exceeds its bound")
	}
	if err := writeExclusiveGenerationFile(filepath.Join(directory, name), body, 0600, 0); err != nil {
		return err
	}
	return syncDirectory(directory)
}
