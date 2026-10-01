//go:build linux

package installation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

func validateSuccessorJournal(root string, intent transitionIntent, files map[string][]byte) error {
	directory := filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	if err := requirePrivateJournalDirectory(directory); err != nil {
		return err
	}
	if err := verifyGenerationDirectory(directory, filepath.Join(root, "generations", intent.Candidate.GenerationDigest), intent.Candidate); err != nil {
		return err
	}
	if err := requirePrivateJournalFile(filepath.Join(directory, "generation-directory.json")); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > 12 {
		return errors.New("repair-required: successor journal inventory is unavailable")
	}
	allowed := map[string]string{"0001.json": "writing-generation", "0002.json": "generation-staged",
		"stop-completed.json": "predecessor-joined", "fixed-completed.json": "successor-fixed-resources-installed",
		"selection-completed.json": "successor-selected", "manager-reloaded.json": "successor-manager-reloaded",
		"start-observed.json": "successor-start-observed", "recovery-staged.json": "fresh-authorized-generation-repaired",
		"original-transition-failure.json": "successor-transition-failed"}
	first := false
	for _, entry := range entries {
		if entry.Name() == "completed-intent.json" {
			if err := requirePrivateJournalFile(filepath.Join(directory, entry.Name())); err != nil {
				return err
			}
			body, err := readInstalledFile(filepath.Join(directory, entry.Name()), 128<<10)
			wanted, encodeErr := canonicalJSON(intent)
			if err != nil || encodeErr != nil || digestHex(body) != digestHex(wanted) {
				return errors.New("repair-required: archived successor intent differs")
			}
			continue
		}
		if entry.Name() == "generation-directory.json" {
			continue
		}
		if entry.Name() == "replacements" {
			if err := requirePrivateJournalDirectory(filepath.Join(directory, entry.Name())); err != nil {
				return err
			}
			continue
		}
		phase, known := allowed[entry.Name()]
		if !known {
			return errors.New("repair-required: successor journal has foreign entries")
		}
		path := filepath.Join(directory, entry.Name())
		if err := requirePrivateJournalFile(path); err != nil {
			return err
		}
		body, err := readInstalledFile(path, 64<<10)
		if err != nil {
			return err
		}
		var record transitionRecord
		if err := decodeCanonical(body, 64<<10, &record); err != nil || record.Schema != "ardents-endpoint-installation-transition-v1" ||
			record.GenerationDigest != intent.Candidate.GenerationDigest || record.BindingDigest != intent.Candidate.BindingDigest {
			return errors.New("repair-required: successor phase binding differs")
		}
		if entry.Name() == "0002.json" && record.Phase == "generation-write-failed" && record.OriginalError != "" {
			continue
		}
		if record.Phase != phase || (entry.Name() != "original-transition-failure.json" && record.OriginalError != "") {
			return errors.New("repair-required: successor phase differs")
		}
		first = first || entry.Name() == "0001.json"
	}
	if !first {
		return errors.New("repair-required: successor has no owned staging origin")
	}
	previous, err := readCandidateBinding(root, intent.Previous)
	if err != nil {
		return err
	}
	// Candidate snapshot is already authenticated by the caller; its binding
	// maps the complete intended bytes even if staging was interrupted.
	candidate := checkedBinding{binding: intent.CandidateBinding, request: intent.Request, files: files}
	return validateReplacementRecords(root, intent, previous, candidate)
}

func validateReplacementRecords(root string, intent transitionIntent, previous, candidate checkedBinding) error {
	directory := filepath.Join(root, "journals", intent.Candidate.GenerationDigest, "replacements")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || len(entries) > 10 {
		return errors.New("repair-required: replacement journal inventory differs")
	}
	oldFiles, err := transitionFixedFiles(previous)
	if err != nil {
		return err
	}
	newFiles, err := transitionFixedFiles(candidate)
	if err != nil {
		return err
	}
	oldFiles[filepath.Join(root, "selection.json")], err = canonicalJSON(intent.Previous)
	if err != nil {
		return err
	}
	newFiles[filepath.Join(root, "selection.json")], err = canonicalJSON(intent.Candidate)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(fixedResources())+1)
	for path := range fixedResources() {
		paths = append(paths, path)
	}
	paths = append(paths, "/etc/ardents/text-worker-artifact.json")
	sort.Strings(paths)
	expected := map[string]string{"selection.json": filepath.Join(root, "selection.json")}
	for index, path := range paths {
		expected[fmt.Sprintf("copy-%04d.json", index+1)] = path
	}
	for _, entry := range entries {
		path, known := expected[entry.Name()]
		if !known {
			return errors.New("repair-required: replacement journal has foreign entries")
		}
		recordPath := filepath.Join(directory, entry.Name())
		if err := requirePrivateJournalFile(recordPath); err != nil {
			return err
		}
		body, err := readInstalledFile(recordPath, 4096)
		if err != nil {
			return err
		}
		var record replacementRecord
		if err := decodeCanonical(body, 4096, &record); err != nil || record.Schema != "ardents-endpoint-installation-replacement-v1" ||
			record.GenerationDigest != intent.Candidate.GenerationDigest || record.Path != path || !canonicalDigest(record.PreviousDigest) || !canonicalDigest(record.CandidateDigest) {
			return errors.New("repair-required: replacement record differs")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.Mode().IsRegular() || identity.Uid != 0 || identity.Nlink != 1 || identity.Gid != record.GID ||
			uint32(info.Mode().Perm()) != record.Mode || uint64(identity.Dev) != record.Device || identity.Ino != record.Inode {
			return errors.New("repair-required: replacement inode differs")
		}
		if path == filepath.Join(root, "selection.json") && record.GID != previous.binding.GID ||
			path != filepath.Join(root, "selection.json") && record.GID != 0 {
			return errors.New("repair-required: replacement group differs")
		}
		mode := uint32(0644)
		if path == filepath.Join(root, "selection.json") {
			mode = 0640
		} else if path == "/usr/lib/ardents/text-worker-root/ardents-text" {
			mode = 0555
		}
		if record.Mode != mode || record.PreviousDigest != digestHex(oldFiles[path]) || record.CandidateDigest != digestHex(newFiles[path]) {
			return errors.New("repair-required: replacement bytes or mode differ from the owned generations")
		}
	}
	return nil
}

func requirePrivateJournalDirectory(path string) error {
	if err := checkRootAncestors(path, false); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || identity.Uid != 0 || identity.Gid != 0 || info.Mode().Perm() != 0700 {
		return errors.New("installation journal directory is not root-private")
	}
	return nil
}

func requirePrivateJournalFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || identity.Uid != 0 || identity.Gid != 0 || identity.Nlink != 1 || info.Mode().Perm() != 0600 {
		return errors.New("installation journal file is not root-private")
	}
	return nil
}
