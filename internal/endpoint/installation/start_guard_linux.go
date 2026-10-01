//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// The guard retains exact recovery provenance independently of cursor archival.
// It is a refusal marker, never evidence that a running invocation is accepted.
func retainStartGuard(root string, intent transitionIntent) error {
	if intent.Request.InstallationRoot != root {
		return errors.New("installation start guard root differs")
	}
	body, err := canonicalJSON(intent)
	if err != nil || len(body) > 128<<10 {
		return errors.New("installation start guard exceeds its bound")
	}
	path := filepath.Join(root, "start-guard.json")
	if current, err := readInstalledFile(path, 128<<10); err == nil {
		if err := requirePrivateJournalFile(path); err != nil {
			return err
		}
		if digestHex(current) != digestHex(body) {
			return errors.New("installation start guard belongs to another transition")
		}
		// A prior failed directory sync is not inferred successful from visibility.
		return syncDirectory(root)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := writeExclusiveGenerationFile(path, body, 0600, 0); err != nil {
		return err
	}
	return syncDirectory(root)
}

func readStartGuard(root string) ([]byte, error) {
	path := filepath.Join(root, "start-guard.json")
	if err := requirePrivateJournalFile(path); err != nil {
		return nil, err
	}
	return readInstalledFile(path, 128<<10)
}

func clearStartSocket(root string, gid uint32) error {
	path := filepath.Join(root, "start-completion.socket")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != 0 || identity.Gid != gid || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0660 {
		return errors.New("installation completion socket ownership differs")
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(root)
}

func clearStartGuard(root string, intent transitionIntent) error {
	current, err := readStartGuard(root)
	if err != nil {
		return err
	}
	wanted, err := canonicalJSON(intent)
	if err != nil || digestHex(current) != digestHex(wanted) {
		return errors.New("installation start guard changed before completion")
	}
	if err := clearStartSocket(root, intent.CandidateBinding.GID); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(root, "start-guard.json")); err != nil {
		return err
	}
	if err := syncDirectory(root); err != nil {
		return errors.Join(err, retainStartGuard(root, intent))
	}
	return nil
}
