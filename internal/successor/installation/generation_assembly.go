package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// These observations are private implementation data. Only native preparation
// may supply them to assembly after checking its owned intent and actual roots.
// They neither serialize nor recreate an opaque Release authorization.
type preparedInstallation struct {
	uid, gid uint32
	roots    []rootIdentity
}

type rootIdentity struct {
	Path   string `json:"path"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type generationSelection struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
}

type targetObservation struct {
	Path            string `json:"path"`
	Digest          string `json:"digest"`
	Length          int64  `json:"length"`
	ReleaseIdentity string `json:"release_identity"`
	ReleaseVersion  int64  `json:"release_version"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	Environment     string `json:"environment"`
	Network         string `json:"network"`
	ReferenceTime   string `json:"reference_time"`
	TargetsVersion  int64  `json:"targets_version"`
	TargetsDigest   string `json:"targets_digest"`
}

type generationBinding struct {
	Schema           string            `json:"schema"`
	InstallationRoot string            `json:"installation_root"`
	ReleaseFloorRoot string            `json:"release_floor_root"`
	GenerationDigest string            `json:"generation_digest"`
	Program          targetObservation `json:"program"`
	Generation       targetObservation `json:"generation"`
	Account          string            `json:"account"`
	Unit             string            `json:"unit"`
	UID              uint32            `json:"uid"`
	GID              uint32            `json:"gid"`
	MutableRoots     []rootIdentity    `json:"mutable_roots"`
	Files            map[string]string `json:"files"`
}

// Assembly produces detached bytes, not a selected or installed generation.
// Native staging owns durable intent, account/root provenance, physical writes
// and their rechecks; it must not infer those facts from a stored binding.
func assembleGeneration(ctx context.Context, request Request, authorization Authorization, prepared preparedInstallation) (map[string][]byte, generationSelection, error) {
	if ctx == nil || request.declared == nil || prepared.uid == 0 || prepared.gid == 0 {
		return nil, generationSelection{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, generationSelection{}, err
	}
	program, programOK := authorization.program.AcceptedDecision()
	generation, generationOK := authorization.generation.AcceptedDecision()
	if !programOK || !generationOK || !request.ReferenceTime().Equal(generation.ReferenceTime) {
		return nil, generationSelection{}, ErrAuthorization
	}
	declared := *request.declared
	rawRequest, err := canonicalJSON(declared)
	if err != nil {
		return nil, generationSelection{}, err
	}
	if _, err := decodeInstallationRequest(rawRequest); err != nil {
		return nil, generationSelection{}, err
	}
	wanted := mutableRoots(declared.Headless)
	if len(wanted) != len(prepared.roots) {
		return nil, generationSelection{}, ErrBinding
	}
	for index, root := range prepared.roots {
		if root.Path != wanted[index] || root.Device == 0 || root.Inode == 0 {
			return nil, generationSelection{}, ErrBinding
		}
	}
	var descriptor generationDeclaration
	if err := json.Unmarshal(authorization.descriptor, &descriptor); err != nil ||
		!coherentTargets(program, generation, descriptor) || len(authorization.resources) != 9 {
		return nil, generationSelection{}, errors.Join(ErrBinding, err)
	}
	if generation.Length != int64(len(authorization.descriptor)) ||
		!bytes.Equal(generation.Digest, digestBytes(authorization.descriptor)) {
		return nil, generationSelection{}, ErrBinding
	}
	files := make(map[string][]byte, 15)
	for name, body := range authorization.resources {
		if descriptor.Files[name] != digestHex(body) {
			return nil, generationSelection{}, ErrBinding
		}
		files[name] = bytes.Clone(body)
	}
	if program.Length != int64(len(files["ardents-linux-amd64"])) ||
		!bytes.Equal(program.Digest, digestBytes(files["ardents-linux-amd64"])) {
		return nil, generationSelection{}, ErrBinding
	}
	digest := digestHex(authorization.descriptor)
	directory := path.Join(declared.InstallationRoot, "generations", digest)
	files["protected-endpoint.json"] = bytes.Clone(authorization.descriptor)
	files["request.json"] = rawRequest
	plan := declared.Headless
	plan.NetworkSourcePlan = path.Join(directory, "source.json")
	files["headless.json"], err = canonicalJSON(plan)
	if err != nil {
		return nil, generationSelection{}, err
	}
	files["source.json"], err = canonicalJSON(declared.Source)
	if err != nil {
		return nil, generationSelection{}, err
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], declared, directory)
	if err != nil {
		return nil, generationSelection{}, err
	}
	binding := generationBinding{Schema: "ardents-endpoint-installation-binding-v1",
		InstallationRoot: declared.InstallationRoot, ReleaseFloorRoot: declared.ReleaseFloorRoot, GenerationDigest: digest,
		Program: observeTarget(program), Generation: observeTarget(generation), Account: "ardents-endpoint", Unit: "ardents-endpoint.service",
		UID: prepared.uid, GID: prepared.gid, MutableRoots: append([]rootIdentity(nil), prepared.roots...), Files: make(map[string]string, len(files))}
	for name, body := range files {
		binding.Files[name] = digestHex(body)
	}
	files["binding.json"], err = canonicalJSON(binding)
	if err != nil {
		return nil, generationSelection{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, generationSelection{}, err
	}
	return files, generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digest, BindingDigest: digestHex(files["binding.json"])}, nil
}

func observeTarget(decision release.Decision) targetObservation {
	return targetObservation{Path: decision.Path, Digest: hex.EncodeToString(decision.Digest), Length: decision.Length,
		ReleaseIdentity: decision.ReleaseIdentity, ReleaseVersion: decision.ReleaseVersion, Platform: decision.Platform,
		Architecture: decision.Architecture, Environment: decision.Environment, Network: decision.Network,
		ReferenceTime: decision.ReferenceTime.UTC().Format(time.RFC3339Nano), TargetsVersion: decision.Floors.TargetsVersion,
		TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest)}
}

func canonicalJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func digestBytes(body []byte) []byte {
	digest := sha256.Sum256(body)
	return digest[:]
}

func digestHex(body []byte) string { return hex.EncodeToString(digestBytes(body)) }
