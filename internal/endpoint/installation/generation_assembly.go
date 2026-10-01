//go:build linux

package installation

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

// assembleGeneration consumes only snapshots retained by fresh authentication.
// Returned binding observations cannot recreate either opaque authorization.
func assembleGeneration(request Request, authorization Authorization, uid, gid uint32, roots []rootBinding) (map[string][]byte, selection, error) {
	program, ok := authorization.program.AcceptedDecision()
	if !ok {
		return nil, selection{}, errors.New("generation assembly lacks executable authorization")
	}
	generation, ok := authorization.generation.AcceptedDecision()
	if !ok {
		return nil, selection{}, errors.New("generation assembly lacks generation authorization")
	}
	rawRequest, err := canonicalJSON(request)
	if err != nil {
		return nil, selection{}, err
	}
	if _, err := decodeRequest(rawRequest); err != nil {
		return nil, selection{}, err
	}
	if uid == 0 || gid == 0 || request.ReferenceTime != generation.ReferenceTime.UTC().Format(time.RFC3339Nano) {
		return nil, selection{}, errors.New("generation assembly account or reference differs")
	}
	wanted := mutableRoots(request.Headless)
	if len(roots) != len(wanted) {
		return nil, selection{}, errors.New("generation assembly mutable roots differ")
	}
	for i, root := range roots {
		if root.Path != wanted[i] || root.Device == 0 || root.Inode == 0 {
			return nil, selection{}, errors.New("generation assembly lacks physical mutable roots")
		}
	}
	digest := digestHex(authorization.descriptor)
	directory := filepath.Join(request.InstallationRoot, "generations", digest)
	files := cloneFiles(authorization.resources)
	files["protected-endpoint.json"] = bytes.Clone(authorization.descriptor)
	files["request.json"] = rawRequest
	plan := request.Headless
	plan.NetworkSourcePlan = filepath.Join(directory, "source.json")
	for name, value := range map[string]any{"headless.json": plan, "source.json": request.Source} {
		files[name], err = canonicalJSON(value)
		if err != nil {
			return nil, selection{}, err
		}
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], request, directory)
	if err != nil {
		return nil, selection{}, err
	}
	binding := localBinding{Schema: "ardents-endpoint-installation-binding-v1", InstallationRoot: request.InstallationRoot,
		ReleaseFloorRoot: request.ReleaseFloorRoot, GenerationDigest: digest, Program: targetObservation(program), Generation: targetObservation(generation),
		Account: "ardents-endpoint", Unit: "ardents-endpoint.service", UID: uid, GID: gid, MutableRoots: append([]rootBinding(nil), roots...), Files: map[string]string{}}
	for name, body := range files {
		binding.Files[name] = digestHex(body)
	}
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		return nil, selection{}, err
	}
	selected := selection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digest, BindingDigest: digestHex(files["binding.json"])}
	// Use the real reader before any writer can persist the assembled candidate.
	selectionBytes, err := canonicalJSON(selected)
	if err != nil {
		return nil, selection{}, err
	}
	_, err = readLocalBinding(request.InstallationRoot, func(path string, maximum int64) ([]byte, error) {
		if path == filepath.Join(request.InstallationRoot, "selection.json") {
			return selectionBytes, nil
		}
		if filepath.Dir(path) != directory {
			return nil, errors.New("generation assembly read escaped candidate")
		}
		body, present := files[filepath.Base(path)]
		if !present || int64(len(body)) > maximum {
			return nil, errors.New("generation assembly candidate file unavailable")
		}
		return body, nil
	})
	if err != nil {
		return nil, selection{}, err
	}
	return files, selected, nil
}

func targetObservation(decision release.Decision) targetBinding {
	return targetBinding{Path: decision.Path, Digest: hex.EncodeToString(decision.Digest), Length: decision.Length, ReleaseIdentity: decision.ReleaseIdentity,
		ReleaseVersion: decision.ReleaseVersion, Platform: decision.Platform, Architecture: decision.Architecture, Environment: decision.Environment,
		Network: decision.Network, ReferenceTime: decision.ReferenceTime.UTC().Format(time.RFC3339Nano), TargetsVersion: decision.Floors.TargetsVersion,
		TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest)}
}
