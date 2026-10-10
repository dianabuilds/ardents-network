// Closed generation binding construction, frozen inventory and independent integrity rules.
package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
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
	programProof, generationProof := authorization.Targets()
	program, programOK := programProof.AcceptedDecision()
	generation, generationOK := generationProof.AcceptedDecision()
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
	descriptorBytes, resources := authorization.Descriptor(), authorization.Resources()
	if err := json.Unmarshal(descriptorBytes, &descriptor); err != nil ||
		!generationauthorization.CoherentTargets(program, generation, descriptor) || len(resources) != 9 {
		return nil, generationSelection{}, errors.Join(ErrBinding, err)
	}
	if generation.Length != int64(len(descriptorBytes)) ||
		!bytes.Equal(generation.Digest, digestBytes(descriptorBytes)) {
		return nil, generationSelection{}, ErrBinding
	}
	files := make(map[string][]byte, 15)
	for name, body := range resources {
		if descriptor.Files[name] != digestHex(body) {
			return nil, generationSelection{}, ErrBinding
		}
		files[name] = bytes.Clone(body)
	}
	if program.Length != int64(len(files["ardents-linux-amd64"])) ||
		!bytes.Equal(program.Digest, digestBytes(files["ardents-linux-amd64"])) {
		return nil, generationSelection{}, ErrBinding
	}
	digest := digestHex(descriptorBytes)
	directory := path.Join(declared.InstallationRoot, "generations", digest)
	files["protected-endpoint.json"] = bytes.Clone(descriptorBytes)
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

// Freeze a closed artifact inventory before native staging. These bytes and
// public binding facts grant no fresh Release or native ownership authority.
func freezeGenerationInventory(request Request, root string, files map[string][]byte, selected generationSelection, gid uint32) (map[string][]byte, generationBinding, error) {
	if request.declared == nil || gid == 0 || selected.Schema != "ardents-endpoint-installation-selection-v1" ||
		!canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest) ||
		selected.BindingDigest != digestHex(files["binding.json"]) {
		return nil, generationBinding{}, ErrBinding
	}
	var binding generationBinding
	if err := json.Unmarshal(files["binding.json"], &binding); err != nil {
		return nil, binding, err
	}
	body, err := canonicalJSON(binding)
	if err != nil || !bytes.Equal(body, files["binding.json"]) || binding.InstallationRoot != root ||
		request.declared.InstallationRoot != root || binding.GenerationDigest != selected.GenerationDigest ||
		selected.GenerationDigest != digestHex(files["protected-endpoint.json"]) || binding.UID == 0 || binding.GID != gid || len(files) != 15 || len(binding.Files) != 14 {
		return nil, binding, ErrBinding
	}
	allowed := map[string]bool{}
	for _, name := range generation.Names() {
		allowed[name] = true
	}
	frozen := make(map[string][]byte, len(files))
	for name, body := range files {
		if !allowed[name] || (name != "binding.json" && binding.Files[name] != digestHex(body)) {
			return nil, binding, ErrBinding
		}
		frozen[name] = bytes.Clone(body)
	}
	return frozen, binding, nil
}

// Initial and successor recovery share exact-byte proof matching, not intent
// admission or native effects. Each caller separately verifies its own schema,
// physical provenance, phase and original process lifetime.
func recoverBoundGeneration(ctx context.Context, selected generationSelection, binding generationBinding, request installationRequest, reference time.Time, authorization Authorization) (inspectedGeneration, error) {
	if err := ctx.Err(); err != nil {
		return inspectedGeneration{}, err
	}
	programProof, generationProof := authorization.Targets()
	p, programOK := programProof.AcceptedDecision()
	g, generationOK := generationProof.AcceptedDecision()
	if !programOK || !generationOK || !p.ReferenceTime.Equal(reference) || !g.ReferenceTime.Equal(reference) {
		return inspectedGeneration{}, ErrAuthorization
	}
	for _, pair := range []struct{ old, fresh targetObservation }{
		{binding.Program, observeTarget(p)}, {binding.Generation, observeTarget(g)},
	} {
		old, fresh := pair.old, pair.fresh
		before, err := time.Parse(time.RFC3339Nano, old.ReferenceTime)
		if err != nil || reference.Before(before) || old.Path != fresh.Path || old.Digest != fresh.Digest || old.Length != fresh.Length ||
			old.ReleaseIdentity != fresh.ReleaseIdentity || old.ReleaseVersion != fresh.ReleaseVersion || old.Platform != fresh.Platform || old.Architecture != fresh.Architecture ||
			old.Environment != fresh.Environment || old.Network != fresh.Network || fresh.TargetsVersion < old.TargetsVersion ||
			(fresh.TargetsVersion == old.TargetsVersion && fresh.TargetsDigest != old.TargetsDigest) {
			return inspectedGeneration{}, ErrBinding
		}
	}
	files := make(map[string][]byte, 14)
	for name, body := range authorization.Resources() {
		files[name] = bytes.Clone(body)
	}
	files["protected-endpoint.json"] = authorization.Descriptor()
	var err error
	files["request.json"], err = canonicalJSON(request)
	if err != nil {
		return inspectedGeneration{}, err
	}
	plan := request.Headless
	plan.NetworkSourcePlan = path.Join(request.InstallationRoot, "generations", selected.GenerationDigest, "source.json")
	files["headless.json"], err = canonicalJSON(plan)
	if err != nil {
		return inspectedGeneration{}, err
	}
	files["source.json"], err = canonicalJSON(request.Source)
	if err != nil {
		return inspectedGeneration{}, err
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], request, path.Join(request.InstallationRoot, "generations", selected.GenerationDigest))
	if err != nil {
		return inspectedGeneration{}, err
	}
	selectedBody, selectionErr := canonicalJSON(selected)
	bindingBody, bindingErr := canonicalJSON(binding)
	if err := errors.Join(selectionErr, bindingErr, ctx.Err()); err != nil {
		return inspectedGeneration{}, err
	}
	return inspectGeneration(request.InstallationRoot, selectedBody, bindingBody, files)
}

func decodeCanonical(raw []byte, maximum int, target any) error {
	if len(raw) == 0 || len(raw) > maximum {
		return errors.New("installation document exceeds its bound")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("installation document is invalid")
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("installation document is not canonical")
	}
	return nil
}

func canonicalDigest(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == 32 && hex.EncodeToString(digest) == value
}

func canonicalPath(value string) bool {
	return path.IsAbs(value) && path.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func pathsOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

type inspectedGeneration struct {
	selected generationSelection
	binding  generationBinding
	request  installationRequest
	files    map[string][]byte
}

// These root-controlled observations cannot recreate private authorizations.
// Enrollment alone checks the canonical protected descriptor/resource group.
func inspectGeneration(root string, selectedRaw, bindingRaw []byte, files map[string][]byte) (inspectedGeneration, error) {
	var selected generationSelection
	var binding generationBinding
	if !canonicalPath(root) || decodeCanonical(selectedRaw, 4<<10, &selected) != nil ||
		selected.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest) ||
		digestHex(bindingRaw) != selected.BindingDigest || decodeCanonical(bindingRaw, 32<<10, &binding) != nil ||
		binding.Schema != "ardents-endpoint-installation-binding-v1" || binding.InstallationRoot != root || binding.GenerationDigest != selected.GenerationDigest ||
		binding.Account != "ardents-endpoint" || binding.Unit != "ardents-endpoint.service" || binding.UID == 0 || binding.GID == 0 || binding.UID == ^uint32(0) || binding.GID == ^uint32(0) ||
		len(binding.Files) != 14 || len(files) != 14 {
		return inspectedGeneration{}, ErrBinding
	}
	for name, body := range files {
		if path.Base(name) != name || !canonicalDigest(binding.Files[name]) || digestHex(body) != binding.Files[name] {
			return inspectedGeneration{}, ErrBinding
		}
	}
	var descriptor struct {
		Schema          string            `json:"schema"`
		Platform        string            `json:"platform"`
		ReleaseIdentity string            `json:"release_identity"`
		ReleaseVersion  int64             `json:"release_version"`
		Files           map[string]string `json:"files"`
	}
	if decodeCanonical(files["protected-endpoint.json"], 16<<10, &descriptor) != nil {
		return inspectedGeneration{}, ErrBinding
	}
	resources := make(map[string][]byte, len(descriptor.Files))
	for name := range descriptor.Files {
		resources[name] = files[name]
	}
	if err := enrollment.ValidateProtectedGeneration(files["protected-endpoint.json"], resources, binding.Program.ReleaseIdentity); err != nil {
		return inspectedGeneration{}, err
	}
	request, err := decodeInstallationRequest(files["request.json"])
	if err != nil || request.InstallationRoot != root || request.ReleaseFloorRoot != binding.ReleaseFloorRoot {
		return inspectedGeneration{}, ErrBinding
	}
	wantedRoots := mutableRoots(request.Headless)
	if len(wantedRoots) != len(binding.MutableRoots) {
		return inspectedGeneration{}, ErrBinding
	}
	for index, identity := range binding.MutableRoots {
		if identity.Path != wantedRoots[index] || identity.Device == 0 || identity.Inode == 0 {
			return inspectedGeneration{}, ErrBinding
		}
	}
	p, err := binding.Program.observation()
	if err != nil {
		return inspectedGeneration{}, err
	}
	g, err := binding.Generation.observation()
	if err != nil || !generationauthorization.CoherentTargets(p, g, generationDeclaration{Platform: descriptor.Platform, ReleaseIdentity: descriptor.ReleaseIdentity, ReleaseVersion: descriptor.ReleaseVersion}) ||
		p.Architecture != "amd64" || p.Platform != "linux-amd64" || binding.Program.ReferenceTime != request.ReferenceTime ||
		p.Length != int64(len(files["ardents-linux-amd64"])) || binding.Program.Digest != digestHex(files["ardents-linux-amd64"]) ||
		g.Length != int64(len(files["protected-endpoint.json"])) || binding.Generation.Digest != selected.GenerationDigest {
		return inspectedGeneration{}, ErrBinding
	}
	directory := path.Join(root, "generations", selected.GenerationDigest)
	plan := request.Headless
	plan.NetworkSourcePlan = path.Join(directory, "source.json")
	headless, headlessErr := canonicalJSON(plan)
	source, sourceErr := canonicalJSON(request.Source)
	unit, unitErr := renderEndpointUnit(files["ardents-endpoint.service"], request, directory)
	if headlessErr != nil || sourceErr != nil || unitErr != nil || !bytes.Equal(headless, files["headless.json"]) || !bytes.Equal(source, files["source.json"]) || !bytes.Equal(unit, files["endpoint-unit.service"]) {
		return inspectedGeneration{}, ErrBinding
	}
	return inspectedGeneration{selected: selected, binding: binding, request: request, files: files}, nil
}

func (target targetObservation) observation() (release.Decision, error) {
	if !canonicalDigest(target.Digest) || !canonicalDigest(target.TargetsDigest) || target.TargetsVersion < 1 || target.ReleaseVersion < 1 || target.Length < 1 ||
		target.ReleaseIdentity == "" || target.Environment == "" || target.Network == "" {
		return release.Decision{}, ErrBinding
	}
	at, err := time.Parse(time.RFC3339Nano, target.ReferenceTime)
	if err != nil || at.IsZero() || at.UTC().Format(time.RFC3339Nano) != target.ReferenceTime {
		return release.Decision{}, ErrBinding
	}
	digest, _ := hex.DecodeString(target.Digest)
	floor, _ := hex.DecodeString(target.TargetsDigest)
	return release.Decision{Path: target.Path, Digest: digest, Length: target.Length, ReleaseIdentity: target.ReleaseIdentity, ReleaseVersion: target.ReleaseVersion,
		Platform: target.Platform, Architecture: target.Architecture, Environment: target.Environment, Network: target.Network, ReferenceTime: at,
		Floors: release.FloorSet{TargetsVersion: target.TargetsVersion, TargetsDigest: floor}}, nil
}
