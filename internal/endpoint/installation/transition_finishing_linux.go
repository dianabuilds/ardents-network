//go:build linux

package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func finishInstalledTransition(ctx context.Context, intent transitionIntent, previous, candidate checkedBinding) (result ProvisionResult, returnedErr error) {
	root, selected := intent.Request.InstallationRoot, intent.Candidate
	journal := filepath.Join(root, "journals", selected.GenerationDigest)
	if err := observeAccountAndRoots(candidate, false); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeStoppedInstallation(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := refuseRemainingInstallationScopes(); err != nil {
		return ProvisionResult{}, err
	}
	if err := recordSuccessorPhase(journal, "stop-completed.json", selected, "predecessor-joined"); err != nil {
		return ProvisionResult{}, err
	}
	// A failure after start must retire the observed successor as well. Before
	// start, all scopes are already joined and no accepting mixed path exists.
	started := false
	acknowledged := false
	defer func() {
		if returnedErr != nil && acknowledged {
			result.Status = "installed-started-recovery-required"
			returnedErr = errors.Join(errors.New("installed invocation acknowledged; post-acceptance cleanup requires recovery"), returnedErr, retainStartGuard(root, intent))
			return
		}
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, restoreTransitionIntent(intent))
		}
		if returnedErr != nil && started {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			returnedErr = errors.Join(returnedErr, stopInstalledPredecessor(cleanup, candidate))
			result = ProvisionResult{}
		}
	}()
	mutations := filepath.Join(journal, "replacements")
	if _, err := os.Lstat(mutations); os.IsNotExist(err) {
		if err := createRootDirectory(mutations, 0700, 0); err != nil {
			return ProvisionResult{}, err
		}
	} else if err != nil {
		return ProvisionResult{}, err
	}
	if err := requirePrivateJournalDirectory(mutations); err != nil {
		return ProvisionResult{}, err
	}
	oldFiles, err := transitionFixedFiles(previous)
	if err != nil {
		return ProvisionResult{}, err
	}
	newFiles, err := transitionFixedFiles(candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	paths := make([]string, 0, len(newFiles))
	for path := range newFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for index, path := range paths {
		mode := os.FileMode(0644)
		if path == "/usr/lib/ardents/text-worker-root/ardents-text" {
			mode = 0555
		}
		if err := replaceTransitionFile(ctx, path, oldFiles[path], newFiles[path], mode, 0,
			filepath.Join(mutations, fmt.Sprintf("copy-%04d.json", index+1)), selected); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := observeFixedResources(candidate); err != nil {
		return ProvisionResult{}, err
	}
	if err := recordSuccessorPhase(journal, "fixed-completed.json", selected, "successor-fixed-resources-installed"); err != nil {
		return ProvisionResult{}, err
	}
	oldSelection, err := canonicalJSON(intent.Previous)
	if err != nil {
		return ProvisionResult{}, err
	}
	newSelection, err := canonicalJSON(selected)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := replaceTransitionFile(ctx, filepath.Join(root, "selection.json"), oldSelection, newSelection, 0640, candidate.binding.GID,
		filepath.Join(mutations, "selection.json"), selected); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeBinding(candidate); err != nil {
		return ProvisionResult{}, err
	}
	if err := recordSuccessorPhase(journal, "selection-completed.json", selected, "successor-selected"); err != nil {
		return ProvisionResult{}, err
	}
	if _, err := runInstallationManager(ctx, "daemon-reload"); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeStoppedInstallation(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := recordSuccessorPhase(journal, "manager-reloaded.json", selected, "successor-manager-reloaded"); err != nil {
		return ProvisionResult{}, err
	}
	// Retain the durable cursor through process start and observation. The bound
	// Endpoint waits at its pre-composition barrier until this owner archives it;
	// process death before acceptance leaves that barrier closed and recoverable.
	completion, err := prepareStartCompletion(ctx, root, intent, candidate.binding.GID)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, completion.close()) }()
	started = true // A failed start request may still have created a process.
	if _, err := runInstallationManager(ctx, "--system", "--no-ask-password", "--no-pager", "start", "ardents-endpoint.service"); err != nil {
		return ProvisionResult{}, err
	}
	unit, service, err := worker.ReadEndpointProperties(ctx)
	if err != nil {
		return ProvisionResult{}, err
	}
	pid, invocation, err := installedInvocation(unit, service)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := verifyInstalledProcess(ctx, unit, service, candidate, pid, invocation); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeInstalledSockets(ctx, unit); err != nil {
		return ProvisionResult{}, err
	}
	connection, err := completion.accept(ctx, selected, pid, candidate.binding.UID, invocation)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, connection.Close()) }()
	if err := recordSuccessorPhase(journal, "start-observed.json", selected, "successor-start-observed"); err != nil {
		return ProvisionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProvisionResult{}, err
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		return ProvisionResult{}, err
	}
	if err := sendStartCompletion(ctx, connection, selected, invocation); err != nil {
		return ProvisionResult{}, err
	}
	acknowledged = true
	result = ProvisionResult{Status: "installed-started", GenerationDigest: selected.GenerationDigest, Role: candidate.result.Role}
	if err := clearStartGuard(root, intent); err != nil {
		return result, err
	}
	return result, nil
}

func transitionFixedFiles(checked checkedBinding) (map[string][]byte, error) {
	files, digests := map[string][]byte{}, map[string]string{}
	for path, name := range fixedResources() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := checked.files[name]
		if len(body) == 0 {
			return nil, errors.New("transition resource is absent")
		}
		files[path] = body
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[path] = digestHex(body)
		}
	}
	manifest, err := canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
	if err != nil {
		return nil, err
	}
	files["/etc/ardents/text-worker-artifact.json"] = manifest
	return files, nil
}

func recordSuccessorPhase(directory, name string, selected selection, phase string) error {
	body, err := readInstalledFile(filepath.Join(directory, name), 64<<10)
	if os.IsNotExist(err) {
		return appendGenerationRecord(directory, name, selected, phase, nil)
	}
	if err != nil {
		return err
	}
	var record transitionRecord
	if err := decodeCanonical(body, 64<<10, &record); err != nil || record.Schema != "ardents-endpoint-installation-transition-v1" ||
		record.GenerationDigest != selected.GenerationDigest || record.BindingDigest != selected.BindingDigest || record.Phase != phase || record.OriginalError != "" {
		return errors.New("successor phase record differs")
	}
	return nil
}

func archiveTransitionIntent(root, journal string, intent transitionIntent) error {
	return archiveTransitionIntentWithSync(root, journal, intent, syncDirectory)
}

func archiveTransitionIntentWithSync(root, journal string, intent transitionIntent, syncDir func(string) error) error {
	archivedFailure := filepath.Join(journal, "original-transition-failure.json")
	if _, err := os.Lstat(archivedFailure); err == nil {
		if err := verifyRetainedTransitionFailure(archivedFailure, intent.Candidate); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	path := filepath.Join(root, "transition.json")
	body, err := readInstalledFile(path, 128<<10)
	if err != nil {
		return err
	}
	wanted, err := canonicalJSON(intent)
	if err != nil || digestHex(body) != digestHex(wanted) {
		return errors.New("completed successor intent changed")
	}
	completed := false
	if _, err := os.Lstat(filepath.Join(journal, "completed-intent.json")); !os.IsNotExist(err) {
		archived, readErr := readInstalledFile(filepath.Join(journal, "completed-intent.json"), 128<<10)
		if err != nil || readErr != nil || digestHex(archived) != digestHex(wanted) {
			return errors.New("completed successor archive differs or is unavailable")
		}
		completed = true
	}
	failure := filepath.Join(root, "transition-failure.json")
	if _, err := os.Lstat(failure); err == nil {
		if _, err := os.Lstat(filepath.Join(journal, "original-transition-failure.json")); !os.IsNotExist(err) {
			return errors.New("original successor failure archive already exists or is unavailable")
		}
		if err := os.Rename(failure, filepath.Join(journal, "original-transition-failure.json")); err != nil {
			return err
		}
		if err := errors.Join(syncDir(journal), syncDir(root)); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if completed {
		if err := os.Remove(path); err != nil {
			return err
		}
	} else {
		if err := os.Rename(path, filepath.Join(journal, "completed-intent.json")); err != nil {
			return err
		}
	}
	return errors.Join(syncDir(journal), syncDir(root))
}
