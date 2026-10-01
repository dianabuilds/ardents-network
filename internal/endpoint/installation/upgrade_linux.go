//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

type transitionIntent struct {
	Schema           string       `json:"schema"`
	Previous         selection    `json:"previous"`
	Candidate        selection    `json:"candidate"`
	CandidateBinding localBinding `json:"candidate_binding"`
	Request          Request      `json:"request"`
}

func restoreTransitionIntent(intent transitionIntent) error {
	root := intent.Request.InstallationRoot
	path := filepath.Join(root, "transition.json")
	body, err := canonicalJSON(intent)
	if err != nil || len(body) > 128<<10 {
		return errors.New("installation transition intent exceeds its bound")
	}
	if existing, err := readInstalledFile(path, 128<<10); err == nil {
		if err := requirePrivateJournalFile(path); err != nil || digestHex(existing) != digestHex(body) {
			return errors.New("installation transition intent already differs")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := writeExclusiveGenerationFile(path, body, 0600, 0); err != nil {
		return err
	}
	return syncDirectory(root)
}

func upgradeInstalled(ctx context.Context, path string) (result ProvisionResult, returnedErr error) {
	if os.Geteuid() != 0 {
		return ProvisionResult{}, errors.New("Endpoint upgrade requires root")
	}
	lease, err := acquireRootLease("/run/ardents-installation.lock")
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, lease.Close())
		if returnedErr != nil && result.Status == "" {
			result = ProvisionResult{}
		}
	}()
	request, previous, authorization, err := loadUpgradeInput(ctx, path)
	if err != nil {
		return ProvisionResult{}, err
	}
	files, selected, err := assembleGeneration(request, authorization, previous.binding.UID, previous.binding.GID, previous.binding.MutableRoots)
	if err != nil {
		return ProvisionResult{}, err
	}
	var binding localBinding
	if err := decodeCanonical(files["binding.json"], 64<<10, &binding); err != nil {
		return ProvisionResult{}, err
	}
	oldBytes, err := readInstalledFile(filepath.Join(request.InstallationRoot, "selection.json"), 4096)
	if err != nil {
		return ProvisionResult{}, err
	}
	var oldSelection selection
	if err := decodeCanonical(oldBytes, 4096, &oldSelection); err != nil {
		return ProvisionResult{}, err
	}
	intent := transitionIntent{Schema: "ardents-endpoint-installation-successor-v1", Previous: oldSelection,
		Candidate: selected, CandidateBinding: binding, Request: request}
	intentBytes, err := canonicalJSON(intent)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := writeExclusiveGenerationFile(filepath.Join(request.InstallationRoot, "transition.json"), intentBytes, 0600, 0); err != nil {
		return ProvisionResult{}, err
	}
	if err := syncDirectory(request.InstallationRoot); err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		if returnedErr != nil {
			if result.Status != "" {
				returnedErr = errors.Join(returnedErr, retainTransitionFailure(request.InstallationRoot, selected, returnedErr))
				return
			}
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			active, readErr := readLocalBinding(request.InstallationRoot, readInstalledFile)
			if readErr == nil {
				returnedErr = errors.Join(returnedErr, stopObservedInstallation(cleanup, active))
			} else {
				// During a torn selection no replacement start is permitted. Keep
				// observing the predecessor rather than adopting partial bytes.
				returnedErr = errors.Join(returnedErr, stopObservedInstallation(cleanup, previous), readErr)
			}
			returnedErr = errors.Join(returnedErr, retainTransitionFailure(request.InstallationRoot, selected, returnedErr))
		}
	}()
	staged, err := writeGeneration(ctx, request.InstallationRoot, authorization, request, binding.UID, binding.GID, binding.MutableRoots)
	if err != nil {
		return ProvisionResult{}, err
	}
	if staged != selected {
		return ProvisionResult{}, errors.New("staged successor differs from transition intent")
	}
	candidate, err := readCandidateBinding(request.InstallationRoot, selected)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := stopInstalledPredecessor(ctx, previous); err != nil {
		return ProvisionResult{}, err
	}
	return finishInstalledTransition(ctx, intent, previous, candidate)
}

func loadUpgradeInput(ctx context.Context, path string) (Request, checkedBinding, Authorization, error) {
	body, err := readInstalledFile(path, 64<<10)
	if err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	request, err := decodeRequest(body)
	if err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	if request.ManifestSHA256 != "" {
		return Request{}, checkedBinding{}, Authorization{}, errors.New("initial manifest pin cannot authorize an upgrade")
	}
	previous, err := readLocalBinding(request.InstallationRoot, readInstalledFile)
	if err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	if err := validateSuccessorPaths(request, previous); err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	for _, name := range []string{"transition.json", "transition-failure.json", "start-guard.json", "start-completion.socket"} {
		if _, err := os.Lstat(filepath.Join(request.InstallationRoot, name)); !os.IsNotExist(err) {
			return Request{}, checkedBinding{}, Authorization{}, errors.New("installation requires explicit transition recovery")
		}
	}
	if err := observeBinding(previous); err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	authorization, err := authenticateBundleSuccessor(ctx, request, previous.binding)
	if err != nil {
		return Request{}, checkedBinding{}, Authorization{}, err
	}
	_, proof := authorization.Targets()
	decision, ok := proof.AcceptedDecision()
	if !ok || decision.ReleaseVersion <= previous.binding.Generation.ReleaseVersion || digestHex(authorization.descriptor) == previous.binding.GenerationDigest {
		return Request{}, checkedBinding{}, Authorization{}, errors.New("upgrade requires a strictly newer distinct protected generation")
	}
	return request, previous, authorization, nil
}

func validateSuccessorPaths(request Request, previous checkedBinding) error {
	if request.InstallationRoot != previous.binding.InstallationRoot || request.ReleaseFloorRoot != previous.binding.ReleaseFloorRoot ||
		request.Headless.Role != previous.request.Headless.Role || request.Headless.NetworkID != previous.request.Headless.NetworkID || !slices.Equal(mutableRoots(request.Headless), mutableRoots(previous.request.Headless)) ||
		!slices.Equal(writableDirectories(request), writableDirectories(previous.request)) {
		return errors.New("successor changes the installed role or durable roots")
	}
	if err := checkRootAncestors(request.BundleRoot, false); err != nil {
		return err
	}
	return checkRootAncestors(request.ReleaseFloorRoot, false)
}

func authenticateBundleSuccessor(ctx context.Context, request Request, previous localBinding) (Authorization, error) {
	at, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil {
		return Authorization{}, err
	}
	candidate, err := enrollment.ReadHeadlessCandidate(request.BundleRoot, filepath.Join(request.BundleRoot, "ardents-linux-amd64"), at)
	if err != nil {
		return Authorization{}, err
	}
	verifier, err := release.Open(request.ReleaseFloorRoot)
	if err != nil {
		return Authorization{}, err
	}
	authorization, authErr := authenticateSuccessor(ctx, verifier, candidate, previous)
	if err := errors.Join(authErr, verifier.Close()); err != nil {
		return Authorization{}, err
	}
	return authorization, nil
}

func readCandidateBinding(root string, selected selection) (checkedBinding, error) {
	body, err := canonicalJSON(selected)
	if err != nil {
		return checkedBinding{}, err
	}
	return readLocalBinding(root, func(path string, maximum int64) ([]byte, error) {
		if path == filepath.Join(root, "selection.json") {
			return body, nil
		}
		return readInstalledFile(path, maximum)
	})
}

func retainTransitionFailure(root string, selected selection, original error) error {
	path := filepath.Join(root, "transition-failure.json")
	archive := filepath.Join(root, "journals", selected.GenerationDigest, "original-transition-failure.json")
	if _, err := os.Lstat(archive); err == nil {
		return verifyRetainedTransitionFailure(archive, selected)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		// Explicit recovery retains the first failure file. It must not erase it
		// or replace it with a later attempt's outcome.
		return verifyRetainedTransitionFailure(path, selected)
	} else if !os.IsNotExist(err) {
		return err
	}
	return appendGenerationRecord(root, "transition-failure.json", selected, "successor-transition-failed", original)
}

func verifyRetainedTransitionFailure(path string, selected selection) error {
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
		record.Phase != "successor-transition-failed" || record.OriginalError == "" {
		return errors.New("retained successor failure differs")
	}
	return nil
}
