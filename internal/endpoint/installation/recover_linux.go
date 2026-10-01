//go:build linux

package installation

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func recoverInstalled(ctx context.Context, root string) (result ProvisionResult, returnedErr error) {
	if os.Geteuid() != 0 || !canonicalPath(root) {
		return ProvisionResult{}, errors.New("Endpoint recovery requires root and canonical installation path")
	}
	lease, err := acquireRootLease("/run/ardents-installation.lock")
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, lease.Close())
		if returnedErr != nil {
			result = ProvisionResult{}
		}
	}()
	intent, err := readTransitionIntent(root)
	if err != nil {
		return ProvisionResult{}, err
	}
	previous, err := readCandidateBinding(root, intent.Previous)
	if err != nil {
		return ProvisionResult{}, err
	}
	if intent.CandidateBinding.Generation.ReleaseVersion <= previous.binding.Generation.ReleaseVersion || intent.Candidate.GenerationDigest == intent.Previous.GenerationDigest {
		return ProvisionResult{}, errors.New("repair-required: intent is not the strictly newer selected successor")
	}
	if intent.CandidateBinding.UID != previous.binding.UID || intent.CandidateBinding.GID != previous.binding.GID ||
		intent.CandidateBinding.ReleaseFloorRoot != previous.binding.ReleaseFloorRoot || !slices.Equal(intent.CandidateBinding.MutableRoots, previous.binding.MutableRoots) {
		return ProvisionResult{}, errors.New("repair-required: successor changes the installed account or physical roots")
	}
	if err := validateSuccessorPaths(intent.Request, previous); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeAccountAndRoots(previous, false); err != nil {
		return ProvisionResult{}, err
	}
	request := intent.Request
	request.ReferenceTime = time.Now().UTC().Format(time.RFC3339Nano)
	authorization, err := authenticateBundleSuccessor(ctx, request, previous.binding)
	if err != nil {
		return ProvisionResult{}, errors.Join(errors.New("repair-required: no fresh floor-compatible successor authority"), err)
	}
	files, err := recoverGenerationSnapshot(intent, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	// Validate the owned journal and inode provenance before stop or file repair.
	if err := validateSuccessorJournal(root, intent, files); err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, retainTransitionFailure(root, intent.Candidate, returnedErr))
		}
	}()
	// The manager can still expose predecessor or successor ExecStart after an
	// interruption between selection and reload. Both are exact owned bindings;
	// neither becomes permission to start or replay the old Release.
	observed := previous
	if candidate, err := readCandidateBinding(root, intent.Candidate); err == nil {
		unit, service, err := worker.ReadEndpointProperties(ctx)
		if err != nil {
			return ProvisionResult{}, err
		}
		if pid, invocation, err := installedInvocation(unit, service); err == nil && verifyInstalledProcess(unit, service, candidate, pid, invocation) == nil {
			observed = candidate
		}
	}
	if err := stopObservedInstallation(ctx, observed); err != nil {
		return ProvisionResult{}, err
	}
	if err := repairOwnedGeneration(ctx, root, intent, files); err != nil {
		return ProvisionResult{}, err
	}
	candidate, err := readCandidateBinding(root, intent.Candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	result, err = finishInstalledTransition(ctx, intent, previous, candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	result.Status = "installed-recovered-started"
	return result, nil
}

func readTransitionIntent(root string) (transitionIntent, error) {
	if err := requirePrivateJournalFile(filepath.Join(root, "transition.json")); err != nil {
		return transitionIntent{}, errors.Join(errors.New("repair-required: successor intent is not root-private"), err)
	}
	body, err := readInstalledFile(filepath.Join(root, "transition.json"), 128<<10)
	if err != nil {
		return transitionIntent{}, errors.Join(errors.New("repair-required: no readable owned successor intent"), err)
	}
	var intent transitionIntent
	if err := decodeCanonical(body, 128<<10, &intent); err != nil {
		return transitionIntent{}, err
	}
	bindingBytes, err := canonicalJSON(intent.CandidateBinding)
	if err != nil || intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Request.InstallationRoot != root ||
		intent.Request.ManifestSHA256 != "" || intent.Previous.Schema != "ardents-endpoint-installation-selection-v1" ||
		intent.Candidate.Schema != intent.Previous.Schema || !canonicalDigest(intent.Previous.GenerationDigest) || !canonicalDigest(intent.Previous.BindingDigest) ||
		!canonicalDigest(intent.Candidate.GenerationDigest) || digestHex(bindingBytes) != intent.Candidate.BindingDigest ||
		intent.CandidateBinding.GenerationDigest != intent.Candidate.GenerationDigest || intent.CandidateBinding.InstallationRoot != root {
		return transitionIntent{}, errors.New("repair-required: successor intent binding differs")
	}
	requestBytes, err := canonicalJSON(intent.Request)
	if err != nil {
		return transitionIntent{}, err
	}
	if _, err := decodeRequest(requestBytes); err != nil {
		return transitionIntent{}, err
	}
	return intent, nil
}

func recoverGenerationSnapshot(intent transitionIntent, authorization Authorization) (map[string][]byte, error) {
	program, programOK := authorization.program.AcceptedDecision()
	generation, generationOK := authorization.generation.AcceptedDecision()
	if !programOK || !generationOK || !recoveryTargetMatches(program, intent.CandidateBinding.Program) ||
		!recoveryTargetMatches(generation, intent.CandidateBinding.Generation) || digestHex(authorization.descriptor) != intent.Candidate.GenerationDigest {
		return nil, errors.New("repair-required: fresh authority does not bind the recorded successor")
	}
	files := cloneFiles(authorization.resources)
	files["protected-endpoint.json"] = authorization.descriptor
	var err error
	files["request.json"], err = canonicalJSON(intent.Request)
	if err != nil {
		return nil, err
	}
	plan := intent.Request.Headless
	directory := filepath.Join(intent.Request.InstallationRoot, "generations", intent.Candidate.GenerationDigest)
	plan.NetworkSourcePlan = filepath.Join(directory, "source.json")
	for name, value := range map[string]any{"headless.json": plan, "source.json": intent.Request.Source, "binding.json": intent.CandidateBinding} {
		files[name], err = canonicalJSON(value)
		if err != nil {
			return nil, err
		}
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], intent.Request, directory)
	if err != nil {
		return nil, err
	}
	if len(files) != 15 || len(intent.CandidateBinding.Files) != 14 || digestHex(files["binding.json"]) != intent.Candidate.BindingDigest {
		return nil, errors.New("repair-required: successor snapshot inventory differs")
	}
	for name, expected := range intent.CandidateBinding.Files {
		if digestHex(files[name]) != expected {
			return nil, errors.New("repair-required: successor snapshot bytes differ")
		}
	}
	selectionBytes, err := canonicalJSON(intent.Candidate)
	if err != nil {
		return nil, err
	}
	if _, err := readLocalBinding(intent.Request.InstallationRoot, func(path string, maximum int64) ([]byte, error) {
		if path == filepath.Join(intent.Request.InstallationRoot, "selection.json") {
			return selectionBytes, nil
		}
		body, found := files[filepath.Base(path)]
		if !found || filepath.Dir(path) != directory || int64(len(body)) > maximum {
			return nil, errors.New("recovery snapshot read escaped its complete generation")
		}
		return body, nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

func recoveryTargetMatches(decision release.Decision, recorded targetBinding) bool {
	return decision.Path == recorded.Path && hex.EncodeToString(decision.Digest) == recorded.Digest && decision.Length == recorded.Length &&
		decision.ReleaseIdentity == recorded.ReleaseIdentity && decision.ReleaseVersion == recorded.ReleaseVersion &&
		decision.Platform == recorded.Platform && decision.Architecture == recorded.Architecture &&
		decision.Environment == recorded.Environment && decision.Network == recorded.Network &&
		decision.Floors.TargetsVersion >= recorded.TargetsVersion &&
		(decision.Floors.TargetsVersion != recorded.TargetsVersion || hex.EncodeToString(decision.Floors.TargetsDigest) == recorded.TargetsDigest)
}
