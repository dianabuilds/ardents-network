package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// CheckResult is a read-only local integrity observation. It is not a fresh
// Release authorization, manager identity receipt or participant readiness.
type CheckResult struct {
	Status           string `json:"status"`
	GenerationDigest string `json:"generation_digest"`
	Role             string `json:"role"`
}

// Check observes selected local bytes under the admitted platform without
// opening Release floors, changing files, starting units or granting readiness.
func Check(ctx context.Context, root string) (CheckResult, error) {
	if ctx == nil {
		return CheckResult{}, errors.New("installation observation context is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := observePlatform(bounded); err != nil {
		return CheckResult{}, err
	}
	checked, err := readLocalBinding(root, readInstalledFile)
	if err != nil {
		return CheckResult{}, err
	}
	if err := observeBinding(checked); err != nil {
		return CheckResult{}, err
	}
	if err := bounded.Err(); err != nil {
		return CheckResult{}, err
	}
	return checked.result, nil
}

type selection struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
}

type targetBinding struct {
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

type localBinding struct {
	Schema           string            `json:"schema"`
	InstallationRoot string            `json:"installation_root"`
	ReleaseFloorRoot string            `json:"release_floor_root"`
	GenerationDigest string            `json:"generation_digest"`
	Program          targetBinding     `json:"program"`
	Generation       targetBinding     `json:"generation"`
	Account          string            `json:"account"`
	Unit             string            `json:"unit"`
	UID              uint32            `json:"uid"`
	GID              uint32            `json:"gid"`
	MutableRoots     []rootBinding     `json:"mutable_roots"`
	Files            map[string]string `json:"files"`
}

type rootBinding struct {
	Path   string `json:"path"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type checkedBinding struct {
	binding   localBinding
	request   Request
	result    CheckResult
	files     map[string][]byte
	directory string
}

// readLocalBinding validates root-controlled observations, not opaque proof.
// The platform reader must enforce filesystem ownership and immutable access.
func readLocalBinding(root string, read func(string, int64) ([]byte, error)) (checkedBinding, error) {
	if !canonicalPath(root) {
		return checkedBinding{}, errors.New("installation root is invalid")
	}
	selectedBytes, err := read(filepath.Join(root, "selection.json"), 4<<10)
	if err != nil {
		return checkedBinding{}, err
	}
	var selected selection
	if err := decodeCanonical(selectedBytes, 4<<10, &selected); err != nil {
		return checkedBinding{}, err
	}
	if selected.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest) {
		return checkedBinding{}, errors.New("installation selection is invalid")
	}
	directory := filepath.Join(root, "generations", selected.GenerationDigest)
	bindingBytes, err := read(filepath.Join(directory, "binding.json"), 32<<10)
	if err != nil || digestHex(bindingBytes) != selected.BindingDigest {
		return checkedBinding{}, errors.New("installation binding does not match selection")
	}
	var binding localBinding
	if err := decodeCanonical(bindingBytes, 32<<10, &binding); err != nil {
		return checkedBinding{}, err
	}
	if binding.Schema != "ardents-endpoint-installation-binding-v1" || binding.InstallationRoot != root ||
		binding.GenerationDigest != selected.GenerationDigest || binding.Account != "ardents-endpoint" || binding.Unit != "ardents-endpoint.service" || binding.UID == 0 || binding.GID == 0 {
		return checkedBinding{}, errors.New("installation binding identity is invalid")
	}
	files := make(map[string][]byte)
	names := append(enrollment.ProtectedResourceNames(), "protected-endpoint.json", "headless.json", "source.json", "endpoint-unit.service", "request.json")
	if len(binding.Files) != len(names) {
		return checkedBinding{}, errors.New("installation binding inventory is invalid")
	}
	for _, name := range names {
		maximum := int64(64 << 10)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			maximum = 64 << 20
		}
		body, err := read(filepath.Join(directory, name), maximum)
		if err != nil || !canonicalDigest(binding.Files[name]) || digestHex(body) != binding.Files[name] {
			return checkedBinding{}, fmt.Errorf("installation generation file differs: %s", name)
		}
		files[name] = body
	}
	request, err := decodeRequest(files["request.json"])
	if err != nil {
		return checkedBinding{}, err
	}
	if request.InstallationRoot != root || request.ReleaseFloorRoot != binding.ReleaseFloorRoot {
		return checkedBinding{}, errors.New("installation request does not match binding roots")
	}
	expectedRoots := mutableRoots(request.Headless)
	if len(expectedRoots) != len(binding.MutableRoots) {
		return checkedBinding{}, errors.New("installation mutable roots differ")
	}
	for index, root := range binding.MutableRoots {
		if root.Path != expectedRoots[index] || root.Device == 0 || root.Inode == 0 {
			return checkedBinding{}, errors.New("installation mutable root identity is invalid")
		}
	}
	if digestHex(files["protected-endpoint.json"]) != selected.GenerationDigest {
		return checkedBinding{}, errors.New("installation generation descriptor identity differs")
	}
	resources := make(map[string][]byte)
	for _, name := range enrollment.ProtectedResourceNames() {
		resources[name] = files[name]
	}
	if err := enrollment.ValidateProtectedGeneration(files["protected-endpoint.json"], resources, binding.Program.ReleaseIdentity); err != nil {
		return checkedBinding{}, err
	}
	var descriptor struct {
		Platform        string `json:"platform"`
		ReleaseIdentity string `json:"release_identity"`
		ReleaseVersion  int64  `json:"release_version"`
	}
	if err := json.Unmarshal(files["protected-endpoint.json"], &descriptor); err != nil {
		return checkedBinding{}, err
	}
	program, err := binding.Program.observation()
	if err != nil {
		return checkedBinding{}, err
	}
	generation, err := binding.Generation.observation()
	if err != nil {
		return checkedBinding{}, err
	}
	if err := matchTargets(program, generation, descriptor.ReleaseIdentity, descriptor.ReleaseVersion, descriptor.Platform); err != nil {
		return checkedBinding{}, err
	}
	if binding.Program.Architecture != "amd64" || binding.Program.ReferenceTime != request.ReferenceTime ||
		binding.Program.Length != int64(len(files["ardents-linux-amd64"])) || binding.Generation.Length != int64(len(files["protected-endpoint.json"])) ||
		binding.Program.Digest != digestHex(files["ardents-linux-amd64"]) || binding.Generation.Digest != selected.GenerationDigest {
		return checkedBinding{}, errors.New("installation target observations differ from its bytes")
	}
	plan := request.Headless
	plan.NetworkSourcePlan = filepath.Join(directory, "source.json")
	headlessBytes, err := canonicalJSON(plan)
	if err != nil {
		return checkedBinding{}, err
	}
	sourceBytes, err := canonicalJSON(request.Source)
	if err != nil {
		return checkedBinding{}, err
	}
	if !bytes.Equal(files["headless.json"], headlessBytes) || !bytes.Equal(files["source.json"], sourceBytes) {
		return checkedBinding{}, errors.New("installation rendered declarations differ")
	}
	unit, err := renderEndpointUnit(files["ardents-endpoint.service"], request, directory)
	if err != nil || !bytes.Equal(unit, files["endpoint-unit.service"]) {
		return checkedBinding{}, errors.New("installation rendered Endpoint unit differs from its authenticated template")
	}
	role := request.Headless.Role
	if role == "" {
		role = "publisher"
	}
	return checkedBinding{binding: binding, request: request, files: files, directory: directory,
		result: CheckResult{Status: "local-integrity-verified", GenerationDigest: selected.GenerationDigest, Role: role}}, nil
}

// observation constructs a public, non-authorizing view solely for coherence
// checks. It cannot create release.Authorization or permit a transition.
func (target targetBinding) observation() (release.Decision, error) {
	if !canonicalDigest(target.Digest) || !canonicalDigest(target.TargetsDigest) || target.TargetsVersion < 1 || target.Length < 1 ||
		target.ReleaseIdentity == "" || target.Environment == "" || target.Network == "" {
		return release.Decision{}, errors.New("installation target observation is invalid")
	}
	at, err := time.Parse(time.RFC3339Nano, target.ReferenceTime)
	if err != nil || at.IsZero() || at.UTC().Format(time.RFC3339Nano) != target.ReferenceTime {
		return release.Decision{}, errors.New("installation target reference time is invalid")
	}
	digest, _ := hex.DecodeString(target.Digest)
	targetsDigest, _ := hex.DecodeString(target.TargetsDigest)
	return release.Decision{Path: target.Path, Digest: digest, Length: target.Length, ReleaseIdentity: target.ReleaseIdentity, ReleaseVersion: target.ReleaseVersion,
		Platform: target.Platform, Architecture: target.Architecture, Environment: target.Environment, Network: target.Network, ReferenceTime: at,
		Floors: release.FloorSet{TargetsVersion: target.TargetsVersion, TargetsDigest: targetsDigest}}, nil
}

func digestHex(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	return append(raw, '\n'), err
}
