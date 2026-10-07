package installation

import (
	"bytes"
	"context"
	"encoding/hex"
	"path"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// CheckResult is a read-only local integrity observation, never a fresh Release
// authorization, active invocation receipt or Service readiness.
type CheckResult struct {
	Status           string
	GenerationDigest string
	Role             string
}

// Check inspects the selected generation without opening Release history,
// repairing resources or starting a process. Native readers retain their
// original Installation lease through observation and physical cleanup.
func Check(ctx context.Context, root string) (CheckResult, error) {
	if ctx == nil {
		return CheckResult{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return CheckResult{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return checkInstalled(bounded, root)
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
	if err != nil || !coherentTargets(p, g, generationDeclaration{Platform: descriptor.Platform, ReleaseIdentity: descriptor.ReleaseIdentity, ReleaseVersion: descriptor.ReleaseVersion}) ||
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
