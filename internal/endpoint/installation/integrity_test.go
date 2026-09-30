package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestBindingBytesVerifyWithoutCreatingRuntimeOrAuthority(t *testing.T) {
	root, files, binding := bindingBytesFixture(t)
	checked, err := readLocalBinding(root, fixtureReader(files))
	if err != nil {
		t.Fatal(err)
	}
	if checked.result.Status != "local-integrity-verified" || checked.result.Role != "reader" || checked.result.GenerationDigest != binding.GenerationDigest {
		t.Fatalf("unexpected local observation: %+v", checked.result)
	}
	for _, target := range []targetBinding{checked.binding.Program, checked.binding.Generation} {
		observation, err := target.observation()
		if err != nil {
			t.Fatal(err)
		}
		if _, authorized := observation.Authorization(); authorized {
			t.Fatal("stored target facts created a fresh Release proof")
		}
	}
	for _, path := range append(mutableRoots(checked.request.Headless), root, checked.request.ReleaseFloorRoot) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("byte observation created %s: %v", path, err)
		}
	}
}

func TestBindingBytesRefuseSubstitutionAndRepinnedDeclarationConflicts(t *testing.T) {
	for _, name := range []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "headless.json", "source.json", "endpoint-unit.service"} {
		t.Run("substituted-"+name, func(t *testing.T) {
			root, files, binding := bindingBytesFixture(t)
			files[filepath.Join(root, "generations", binding.GenerationDigest, name)] = []byte("substitute")
			if _, err := readLocalBinding(root, fixtureReader(files)); err == nil {
				t.Fatal("changed generation bytes accepted")
			}
		})
	}
	for _, test := range []struct {
		name   string
		change func(*localBinding, map[string][]byte, string)
	}{
		{"target-version", func(b *localBinding, _ map[string][]byte, _ string) { b.Generation.ReleaseVersion++ }},
		{"target-set", func(b *localBinding, _ map[string][]byte, _ string) { b.Generation.TargetsVersion++ }},
		{"target-path", func(b *localBinding, _ map[string][]byte, _ string) { b.Generation.Path = programTarget }},
		{"mutable-root", func(b *localBinding, _ map[string][]byte, _ string) { b.MutableRoots[0].Path += "-other" }},
		{"account", func(b *localBinding, _ map[string][]byte, _ string) { b.Account = "root" }},
		{"unknown-file", func(b *localBinding, _ map[string][]byte, _ string) { b.Files["extra"] = strings.Repeat("11", 32) }},
		{"rendered-source", func(b *localBinding, files map[string][]byte, directory string) {
			path := filepath.Join(directory, "source.json")
			files[path] = []byte("{}\n")
			b.Files["source.json"] = digestHex(files[path])
		}},
		{"duplicate-request", func(b *localBinding, files map[string][]byte, directory string) {
			path := filepath.Join(directory, "request.json")
			files[path] = bytes.Replace(files[path], []byte(`"reference_time":`), []byte(`"reference_time":"2026-10-01T00:00:00Z","reference_time":`), 1)
			b.Files["request.json"] = digestHex(files[path])
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, files, binding := bindingBytesFixture(t)
			directory := filepath.Join(root, "generations", binding.GenerationDigest)
			test.change(&binding, files, directory)
			publishFixtureBinding(t, root, files, binding)
			if _, err := readLocalBinding(root, fixtureReader(files)); err == nil {
				t.Fatal("repinned local contradiction accepted")
			}
		})
	}
}

func TestInstallationRequestRefusesMixedAuthorityAndOverlappingRoots(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Request)
	}{
		{"Reader-Publisher", func(r *Request) {
			r.Headless.ServiceInstanceRoot = filepath.Join(filepath.Dir(r.InstallationRoot), "instance")
		}},
		{"Source-network", func(r *Request) { r.Source.NetworkID = strings.Repeat("99", 32) }},
		{"Source-clock", func(r *Request) { r.Source.ClockObservationFile += "-other" }},
		{"Source-signers", func(r *Request) { r.Source.AuthorityPublic = []string{strings.Repeat("bb", 32)} }},
		{"no-refresh", func(r *Request) { r.Source.RefreshIntervalMS = 0 }},
		{"external-Source-plan", func(r *Request) {
			r.Headless.NetworkSourcePlan = filepath.Join(filepath.Dir(r.InstallationRoot), "mutable-source.json")
		}},
		{"mutable-in-generation", func(r *Request) { r.Headless.NetworkStateRoot = filepath.Join(r.InstallationRoot, "state") }},
		{"floor-in-generation", func(r *Request) { r.ReleaseFloorRoot = filepath.Join(r.InstallationRoot, "floors") }},
		{"non-UTC-time", func(r *Request) { r.ReferenceTime = "2026-10-01T00:00:00+00:00" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := installationRequestFixture(t)
			test.change(&request)
			raw, err := canonicalJSON(request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeRequest(raw); err == nil {
				t.Fatal("invalid installation request accepted")
			}
		})
	}
	request := installationRequestFixture(t)
	raw, err := canonicalJSON(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{append([]byte(" "), raw...), append(raw, []byte("{}")...), bytes.Repeat([]byte(" "), 64<<10+1), bytes.Replace(raw, []byte(`{"schema":`), []byte(`{"unknown":true,"schema":`), 1)} {
		if _, err := decodeRequest(invalid); err == nil {
			t.Fatal("noncanonical installation envelope accepted")
		}
	}
}

func bindingBytesFixture(t *testing.T) (string, map[string][]byte, localBinding) {
	t.Helper()
	request := installationRequestFixture(t)
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "fixture-floors"))
	if err != nil {
		t.Fatal(err)
	}
	proofs, err := Authenticate(context.Background(), verifier, enrolled)
	closeErr := verifier.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("fixture Release proofs: %v / %v", err, closeErr)
	}
	programProof, generationProof := proofs.Targets()
	program, _ := programProof.AcceptedDecision()
	generation, _ := generationProof.AcceptedDecision()
	binding := localBinding{Schema: "ardents-endpoint-installation-binding-v1", InstallationRoot: request.InstallationRoot, ReleaseFloorRoot: request.ReleaseFloorRoot,
		GenerationDigest: digestHex(enrolled.ProtectedDescriptor), Program: fixtureTargetBinding(program), Generation: fixtureTargetBinding(generation),
		Account: "ardents-endpoint", Unit: "ardents-endpoint.service", UID: 1001, GID: 1001, Files: map[string]string{}}
	for _, path := range mutableRoots(request.Headless) {
		binding.MutableRoots = append(binding.MutableRoots, rootBinding{Path: path, Device: 1, Inode: 1})
	}
	directory := filepath.Join(request.InstallationRoot, "generations", binding.GenerationDigest)
	files := map[string][]byte{}
	for name, body := range enrolled.ProtectedFiles {
		files[filepath.Join(directory, name)] = bytes.Clone(body)
		binding.Files[name] = digestHex(body)
	}
	files[filepath.Join(directory, "protected-endpoint.json")] = bytes.Clone(enrolled.ProtectedDescriptor)
	plan := request.Headless
	plan.NetworkSourcePlan = filepath.Join(directory, "source.json")
	for name, value := range map[string]any{"request.json": request, "headless.json": plan, "source.json": request.Source} {
		body, err := canonicalJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.Join(directory, name)] = body
	}
	// These fixture bytes do not implement or qualify a systemd unit. The
	// platform wrapper additionally checks actual fixed resources and access.
	files[filepath.Join(directory, "endpoint-unit.service")] = []byte("fixture rendered unit\n")
	for _, name := range []string{"protected-endpoint.json", "request.json", "headless.json", "source.json", "endpoint-unit.service"} {
		binding.Files[name] = digestHex(files[filepath.Join(directory, name)])
	}
	publishFixtureBinding(t, request.InstallationRoot, files, binding)
	return request.InstallationRoot, files, binding
}

func publishFixtureBinding(t *testing.T, root string, files map[string][]byte, binding localBinding) {
	t.Helper()
	body, err := canonicalJSON(binding)
	if err != nil {
		t.Fatal(err)
	}
	files[filepath.Join(root, "generations", binding.GenerationDigest, "binding.json")] = body
	selected, err := canonicalJSON(selection{Schema: "ardents-endpoint-installation-selection-v1", GenerationDigest: binding.GenerationDigest, BindingDigest: digestHex(body)})
	if err != nil {
		t.Fatal(err)
	}
	files[filepath.Join(root, "selection.json")] = selected
}

func fixtureTargetBinding(decision release.Decision) targetBinding {
	return targetBinding{Path: decision.Path, Digest: hex.EncodeToString(decision.Digest), Length: decision.Length, ReleaseIdentity: decision.ReleaseIdentity, ReleaseVersion: decision.ReleaseVersion,
		Platform: decision.Platform, Architecture: decision.Architecture, Environment: decision.Environment, Network: decision.Network, ReferenceTime: decision.ReferenceTime.UTC().Format(time.RFC3339Nano),
		TargetsVersion: decision.Floors.TargetsVersion, TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest)}
}

func fixtureReader(files map[string][]byte) func(string, int64) ([]byte, error) {
	return func(path string, maximum int64) ([]byte, error) {
		body, present := files[path]
		if !present || int64(len(body)) > maximum {
			return nil, os.ErrNotExist
		}
		return bytes.Clone(body), nil
	}
}

func installationRequestFixture(t *testing.T) Request {
	t.Helper()
	base := t.TempDir()
	path := func(name string) string { return filepath.Join(base, name) }
	public := strings.Repeat("aa", 32)
	plan := runtimeplan.Headless{Schema: "ardents-headless-runtime-v2", Role: "reader", NetworkStateRoot: path("state"), EntryStateRoot: path("entry"),
		LocalRoleStateRoot: path("roles"), TextTokenRoot: path("tokens"), ApplicationSocket: path("application.sock"), TimeConfidenceFile: path("clock.json"),
		ReaderPermission: runtimeplan.Permission{RequestPath: path("request-public"), ResponsePath: path("response-public"), Maxima: [3]uint32{1}},
		NetworkID:        strings.Repeat("11", 32), NetworkAuthorities: []string{public}, NetworkThreshold: 1, NetworkProfile: "ardents-route-v3", ClosedProfileAuthority: public,
		BrokerID: strings.Repeat("22", 32), ConnectionPrincipal: strings.Repeat("33", 32)}
	source := runtimeplan.Source{Schema: "ardents-source-plan-v1", NetworkID: plan.NetworkID, AuthorityPublic: plan.NetworkAuthorities, Threshold: plan.NetworkThreshold,
		ClockObservedAt: "2026-10-01T00:00:00Z", ClockObservationFile: plan.TimeConfidenceFile, OrderSeed: strings.Repeat("44", 32), RefreshIntervalMS: 1000,
		LocalRoleStateRoot: plan.LocalRoleStateRoot, ClientCertificate: path("client.pem"), ClientKey: path("client.key")}
	for _, identity := range []string{"55", "66"} {
		source.Sources = append(source.Sources, runtimeplan.SourceMember{Address: "127.0.0.1:12345", ServerName: "source.test", Identity: strings.Repeat(identity, 32), Family: identity, EndpointHandle: identity, RootCA: path("ca.pem"), LeafKeyDigest: strings.Repeat("77", 32)})
	}
	pin := sha256.Sum256([]byte("fixture independent manifest"))
	return Request{Schema: "ardents-endpoint-installation-request-v1", BundleRoot: path("bundle"), ManifestSHA256: hex.EncodeToString(pin[:]), InstallationRoot: path("installation"), ReleaseFloorRoot: path("release-floors"), ReferenceTime: "2026-10-01T00:00:00Z", Headless: plan, Source: source}
}
