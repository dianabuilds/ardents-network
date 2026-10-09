package installation

import (
	"bytes"
	"context"
	"errors"
	"path"
	"strings"
	"testing"
)

func inventoryFixture(t *testing.T) (Request, map[string][]byte, generationSelection) {
	t.Helper()
	selected, binding, files := installedObservationFixture(t)
	raw, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	files["binding.json"] = raw
	request, err := DecodeRequest(t.Context(), files["request.json"], true)
	if err != nil {
		t.Fatal(err)
	}
	return request, files, selected
}

func TestGenerationInventoryFreezeRetainsDetachedClosedBytes(t *testing.T) {
	request, files, selected := inventoryFixture(t)
	frozen, _, err := freezeGenerationInventory(request, "/installation", files, selected, 65534)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(frozen["ardents-linux-amd64"])
	files["ardents-linux-amd64"][0] ^= 1
	if !bytes.Equal(frozen["ardents-linux-amd64"], original) {
		t.Fatal("caller changed frozen program bytes")
	}
	for _, change := range []string{"missing", "extra", "changed", "foreign-root", "foreign-group", "binding-digest"} {
		request, files, selected := inventoryFixture(t)
		root, gid := "/installation", uint32(65534)
		switch change {
		case "missing":
			delete(files, "ardents-text-reader.socket")
		case "extra":
			files["foreign"] = []byte("foreign")
		case "changed":
			files["ardents-linux-amd64"][0] ^= 1
		case "foreign-root":
			root = "/foreign"
		case "foreign-group":
			gid = 65533
		case "binding-digest":
			selected.BindingDigest = selected.GenerationDigest
		}
		if frozen, _, err := freezeGenerationInventory(request, root, files, selected, gid); err == nil || frozen != nil {
			t.Fatal("changed staging inventory accepted", change, err)
		}
	}
}

// Public stored observations are deliberately not private Release proofs.
// This fixture tests grammar/coherence only, not native installed acceptance.
func installedObservationFixture(t *testing.T) (generationSelection, generationBinding, map[string][]byte) {
	t.Helper()
	request, err := decodeInstallationRequest(requestFixture())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	resources := map[string]string{}
	for _, name := range strings.Fields("ardents-linux-amd64 ardents-text-linux-amd64 ardents-text-reader@.service ardents-text-publisher@.service ardents-text-reader.socket ardents-text-publisher.socket 50-ardents-text.rules ardents-text.conf ardents-endpoint.service") {
		files[name] = []byte("static component " + name)
		if name == "ardents-endpoint.service" {
			files[name] = []byte(endpointTemplateFixture)
		}
		resources[name] = digestHex(files[name])
	}
	files["protected-endpoint.json"], err = canonicalJSON(struct {
		Schema          string            `json:"schema"`
		Platform        string            `json:"platform"`
		ReleaseIdentity string            `json:"release_identity"`
		ReleaseVersion  int64             `json:"release_version"`
		Files           map[string]string `json:"files"`
	}{"ardents-protected-endpoint-artifact-v1", "linux-amd64", "release", 1, resources})
	if err != nil {
		t.Fatal(err)
	}
	digest := digestHex(files["protected-endpoint.json"])
	directory := path.Join("/installation", "generations", digest)
	files["request.json"] = requestFixture()
	plan := request.Headless
	plan.NetworkSourcePlan = path.Join(directory, "source.json")
	files["headless.json"], err = canonicalJSON(plan)
	if err != nil {
		t.Fatal(err)
	}
	files["source.json"], err = canonicalJSON(request.Source)
	if err != nil {
		t.Fatal(err)
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], request, directory)
	if err != nil {
		t.Fatal(err)
	}
	p := targetObservation{Path: programTarget, Digest: digestHex(files["ardents-linux-amd64"]), Length: int64(len(files["ardents-linux-amd64"])),
		ReleaseIdentity: "release", ReleaseVersion: 1, Platform: "linux-amd64", Architecture: "amd64", Environment: "closed", Network: request.Headless.NetworkID,
		ReferenceTime: request.ReferenceTime, TargetsVersion: 1, TargetsDigest: strings.Repeat("09", 32)}
	g := p
	g.Path, g.Digest, g.Length = generationTarget, digest, int64(len(files["protected-endpoint.json"]))
	binding := generationBinding{Schema: "ardents-endpoint-installation-binding-v1", InstallationRoot: "/installation", ReleaseFloorRoot: "/floors", GenerationDigest: digest,
		Program: p, Generation: g, Account: "ardents-endpoint", Unit: "ardents-endpoint.service", UID: 65534, GID: 65534, Files: map[string]string{}}
	for _, name := range mutableRoots(request.Headless) {
		binding.MutableRoots = append(binding.MutableRoots, rootIdentity{Path: name, Device: 1, Inode: 1})
	}
	for name, body := range files {
		binding.Files[name] = digestHex(body)
	}
	bindingRaw, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	return generationSelection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: digest, BindingDigest: digestHex(bindingRaw)}, binding, files
}

func inspectObservationFixture(t *testing.T, selected generationSelection, binding generationBinding, files map[string][]byte) error {
	t.Helper()
	bindingRaw, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	selected.BindingDigest = digestHex(bindingRaw)
	selectedRaw, err := canonicalJSON(selected)
	if err != nil {
		t.Fatal(err)
	}
	_, err = inspectGeneration("/installation", selectedRaw, bindingRaw, files)
	return err
}

func TestInstalledGenerationStoredObservationCoherence(t *testing.T) {
	s, b, files := installedObservationFixture(t)
	if err := inspectObservationFixture(t, s, b, files); err != nil {
		t.Fatal(err)
	}
	p, err := b.Program.observation()
	if err != nil {
		t.Fatal(err)
	}
	if _, accepting := p.Authorization(); accepting {
		t.Fatal("stored observation minted Release authority")
	}
}

func TestInstalledGenerationRefusesCoherentlyRehashedForeignFacts(t *testing.T) {
	for name, mutate := range map[string]func(*generationBinding, map[string][]byte){
		"foreign-program-path": func(b *generationBinding, _ map[string][]byte) { b.Program.Path = "foreign" },
		"different-floor":      func(b *generationBinding, _ map[string][]byte) { b.Generation.TargetsDigest = strings.Repeat("10", 32) },
		"different-reference":  func(b *generationBinding, _ map[string][]byte) { b.Generation.ReferenceTime = "2030-01-02T03:04:06Z" },
		"foreign-root":         func(b *generationBinding, _ map[string][]byte) { b.MutableRoots[0].Path = "/foreign" },
		"foreign-account":      func(b *generationBinding, _ map[string][]byte) { b.Account = "root" },
		"foreign-group":        func(b *generationBinding, _ map[string][]byte) { b.GID = 0 },
		"foreign-floor-root":   func(b *generationBinding, _ map[string][]byte) { b.ReleaseFloorRoot = "/foreign" },
		"resource-substitution": func(b *generationBinding, f map[string][]byte) {
			f["ardents-text-linux-amd64"] = []byte("foreign bytes")
			b.Files["ardents-text-linux-amd64"] = digestHex(f["ardents-text-linux-amd64"])
		},
		"rendered-unit-substitution": func(b *generationBinding, f map[string][]byte) {
			f["endpoint-unit.service"] = []byte("foreign unit")
			b.Files["endpoint-unit.service"] = digestHex(f["endpoint-unit.service"])
		},
		"rendered-source-substitution": func(b *generationBinding, f map[string][]byte) {
			f["source.json"] = []byte("foreign source")
			b.Files["source.json"] = digestHex(f["source.json"])
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, b, files := installedObservationFixture(t)
			mutate(&b, files)
			if err := inspectObservationFixture(t, s, b, files); err == nil {
				t.Fatal("rehashed foreign installed observation accepted")
			}
		})
	}
}

func TestInstalledCheckRefusesMissingContextAndOriginalCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"missing-context", nil, ErrInput},
		{"original-cancellation", ctx, context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			if result, err := Check(test.ctx, "/installation"); !errors.Is(err, test.want) || result != (CheckResult{}) {
				t.Fatal(result, err)
			}
		})
	}
}
