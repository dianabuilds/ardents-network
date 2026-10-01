//go:build linux

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

// Initial recovery completes an owned installation and leaves it stopped,
// preserving provision's separate explicit-start contract. If creation stopped
// before fixed inode provenance was persisted, repair is required; a pathname
// or initial pin cannot supply the missing ownership or fresh Release proof.
func recoverInitialInstallation(ctx context.Context, intent transitionIntent) (result ProvisionResult, returnedErr error) {
	root := intent.Request.InstallationRoot
	journal := filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	if err := checkRootAncestors(intent.Request.BundleRoot, false); err != nil {
		return ProvisionResult{}, err
	}
	if err := checkRootAncestors(intent.Request.ReleaseFloorRoot, false); err != nil {
		return ProvisionResult{}, err
	}
	request := intent.Request
	request.ReferenceTime = time.Now().UTC().Format(time.RFC3339Nano)
	authorization, err := authenticateBundleSuccessor(ctx, request, intent.CandidateBinding)
	if err != nil {
		return ProvisionResult{}, errors.Join(errors.New("repair-required: initial recovery lacks fresh established-floor authority"), err)
	}
	files, err := recoverGenerationSnapshot(intent, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	candidate := checkedBinding{binding: intent.CandidateBinding, request: intent.Request, files: files}
	if err := observeAccountAndRoots(candidate, false); err != nil {
		return ProvisionResult{}, err
	}
	if err := validateInitialRecoveryJournal(journal, intent); err != nil {
		return ProvisionResult{}, err
	}
	if err := verifyGenerationDirectory(journal, filepath.Join(root, "generations", intent.Candidate.GenerationDigest), intent.Candidate); err != nil {
		return ProvisionResult{}, err
	}
	fixed, err := transitionFixedFiles(candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	fixed[filepath.Join(root, "selection.json")], err = canonicalJSON(intent.Candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	entries, err := os.ReadDir(filepath.Join(journal, "creations"))
	if err != nil || len(entries) != len(fixed) {
		return ProvisionResult{}, errors.New("repair-required: initial fixed resource birth inventory is incomplete")
	}
	wantedRecords := make(map[string]bool, len(fixed))
	for path := range fixed {
		wantedRecords[digestHex([]byte(path))+".json"] = true
	}
	for _, entry := range entries {
		if !wantedRecords[entry.Name()] {
			return ProvisionResult{}, errors.New("repair-required: initial birth journal has foreign entries")
		}
	}
	paths := make([]string, 0, len(fixed))
	for path, body := range fixed {
		mode, gid := initialRecoveryFileMode(path, root, candidate.binding.GID)
		if err := validateCreatedInstallationFile(path, body, mode, gid, journal, intent.Candidate); err != nil {
			return ProvisionResult{}, err
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, retainTransitionFailure(root, intent.Candidate, returnedErr))
		}
	}()
	// Before reload, the initial manager may never have loaded these units.
	// If it has, stop only after checking the exact owned installation identity.
	if err := refuseLoadedInstallationUnits(ctx); err != nil {
		if err := stopObservedInstallation(ctx, candidate); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := refuseRemainingInstallationScopes(); err != nil {
		return ProvisionResult{}, err
	}
	if err := repairOwnedGeneration(ctx, root, intent, files); err != nil {
		return ProvisionResult{}, err
	}
	for _, path := range paths {
		mode, gid := initialRecoveryFileMode(path, root, candidate.binding.GID)
		if err := validateCreatedInstallationFile(path, fixed[path], mode, gid, journal, intent.Candidate); err != nil {
			return ProvisionResult{}, err
		}
		if err := repairCreatedInstallationFile(ctx, path, fixed[path], mode, gid, journal, intent.Candidate); err != nil {
			return ProvisionResult{}, err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return ProvisionResult{}, err
		}
	}
	checked, err := readCandidateBinding(root, intent.Candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := observeBinding(checked); err != nil {
		return ProvisionResult{}, err
	}
	if _, err := runInstallationManager(ctx, "daemon-reload"); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeStoppedInstallation(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := refuseRemainingInstallationScopes(); err != nil {
		return ProvisionResult{}, err
	}
	if err := recordSuccessorPhase(journal, "initial-recovered.json", intent.Candidate, "initial-recovered-stopped"); err != nil {
		return ProvisionResult{}, err
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{Status: "installed-recovered-stopped", GenerationDigest: intent.Candidate.GenerationDigest, Role: checked.request.Headless.Role}, nil
}

func initialRecoveryFileMode(path, root string, endpointGID uint32) (os.FileMode, uint32) {
	if path == filepath.Join(root, "selection.json") {
		return 0640, endpointGID
	}
	if path == "/usr/lib/ardents/text-worker-root/ardents-text" {
		return 0555, 0
	}
	return 0644, 0
}

func validateCreatedInstallationFile(path string, expected []byte, mode os.FileMode, gid uint32, journal string, selected selection) error {
	recordPath := filepath.Join(journal, "creations", digestHex([]byte(path))+".json")
	if err := requirePrivateJournalDirectory(filepath.Dir(recordPath)); err != nil {
		return err
	}
	if err := requirePrivateJournalFile(recordPath); err != nil {
		return errors.Join(errors.New("repair-required: no durable initial resource birth record"), err)
	}
	body, err := readInstalledFile(recordPath, 4096)
	if err != nil {
		return err
	}
	var record replacementRecord
	if err := decodeCanonical(body, 4096, &record); err != nil || record.Schema != "ardents-endpoint-installation-creation-v1" ||
		record.GenerationDigest != selected.GenerationDigest || record.Path != path || record.PreviousDigest != "" ||
		record.CandidateDigest != digestHex(expected) || record.Mode != uint32(mode) || record.GID != gid {
		return errors.New("repair-required: initial resource birth binding differs")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || identity.Uid != 0 || identity.Nlink != 1 ||
		uint64(identity.Dev) != record.Device || identity.Ino != record.Inode ||
		(identity.Gid != 0 && identity.Gid != gid) || (info.Mode().Perm() != 0600 && info.Mode().Perm() != mode) {
		return errors.New("repair-required: initial resource inode differs from its birth record")
	}
	current, err := readInstalledFile(path, int64(len(expected))+1)
	if err != nil || len(current) > len(expected) || !bytes.Equal(current, expected[:len(current)]) {
		return errors.New("repair-required: initial resource is not its authorized prefix")
	}
	return checkRootAncestors(filepath.Dir(path), false)
}

func validateInitialRecoveryJournal(journal string, intent transitionIntent) error {
	selected := intent.Candidate
	if err := requirePrivateJournalDirectory(journal); err != nil {
		return err
	}
	allowed := map[string]string{"0001.json": "writing-generation", "0002.json": "generation-staged",
		"0003.json": "installing-fixed-resources", "0004.json": "fixed-resources-installed", "0005.json": "publishing-selection",
		"0006.json": "reloading-manager", "0007.json": "installed-stopped", "fixed-resource-failure.json": "fixed-resources-failed",
		"selection-failure.json": "selection-failed", "recovery-staged.json": "fresh-authorized-generation-repaired",
		"initial-recovered.json": "initial-recovered-stopped", "original-transition-failure.json": "successor-transition-failed"}
	entries, err := os.ReadDir(journal)
	if err != nil || len(entries) > len(allowed)+3 {
		return errors.New("repair-required: initial journal inventory differs")
	}
	first := false
	for _, entry := range entries {
		if entry.Name() == "completed-intent.json" {
			path := filepath.Join(journal, entry.Name())
			if err := requirePrivateJournalFile(path); err != nil {
				return err
			}
			body, err := readInstalledFile(path, 128<<10)
			wanted, encodeErr := canonicalJSON(intent)
			if err != nil || encodeErr != nil || digestHex(body) != digestHex(wanted) {
				return errors.New("repair-required: archived initial intent differs")
			}
			continue
		}
		if entry.Name() == "creations" {
			if err := requirePrivateJournalDirectory(filepath.Join(journal, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if entry.Name() == "generation-directory.json" {
			if err := requirePrivateJournalFile(filepath.Join(journal, entry.Name())); err != nil {
				return err
			}
			continue
		}
		phase, known := allowed[entry.Name()]
		if !known {
			return errors.New("repair-required: initial journal has foreign entries")
		}
		path := filepath.Join(journal, entry.Name())
		if err := requirePrivateJournalFile(path); err != nil {
			return err
		}
		body, err := readInstalledFile(path, 64<<10)
		if err != nil {
			return err
		}
		var record transitionRecord
		if err := decodeCanonical(body, 64<<10, &record); err != nil || record.Schema != "ardents-endpoint-installation-transition-v1" ||
			record.GenerationDigest != selected.GenerationDigest || record.BindingDigest != selected.BindingDigest ||
			(record.Phase != phase && !(entry.Name() == "0002.json" && record.Phase == "generation-write-failed" && record.OriginalError != "")) {
			return errors.New("repair-required: initial journal phase binding differs")
		}
		first = first || entry.Name() == "0001.json"
	}
	if !first {
		return errors.New("repair-required: initial staging origin is absent")
	}
	return nil
}
