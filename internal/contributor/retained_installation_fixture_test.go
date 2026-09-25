package contributor_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// installRetainedContributorFixture creates authenticated historical state for
// Control tests without invoking the retired Apply/start path.
func installRetainedContributorFixture(t *testing.T, root, bundle, pin string, supervisor *profileSupervisor) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(bundle, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != pin {
		t.Fatal("retained fixture bundle does not match its independent pin")
	}
	var manifest struct {
		Profile      string            `json:"profile"`
		DeploymentID string            `json:"deployment_id"`
		Generation   uint64            `json:"generation"`
		Files        map[string]string `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	programRoot := filepath.Join(root, "usr", "lib", "ardents-contributor")
	privateRoot := filepath.Join(root, "var", "lib", "private", "ardents-contributor")
	programCurrent := filepath.Join(programRoot, "current")
	configCurrent := filepath.Join(privateRoot, "config", "current")
	for _, directory := range []string{programCurrent, configCurrent, filepath.Join(privateRoot, "diagnostics")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range manifest.Files {
		if filepath.Base(name) != name {
			t.Fatalf("invalid retained fixture filename %q", name)
		}
		contents, err := os.ReadFile(filepath.Join(bundle, name))
		if err != nil {
			t.Fatal(err)
		}
		actual := sha256.Sum256(contents)
		if hex.EncodeToString(actual[:]) != want {
			t.Fatalf("retained fixture file %q differs from its manifest", name)
		}
		directory, mode := configCurrent, os.FileMode(0o600)
		if name == "ardents-node" {
			directory, mode = programCurrent, 0o755
		}
		if err := os.WriteFile(filepath.Join(directory, name), contents, mode); err != nil {
			t.Fatal(err)
		}
		if name == "ardents-node" {
			if err := os.WriteFile(filepath.Join(programRoot, name), contents, mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	// These bytes are the fixed unit installed by the retired Apply path.
	// Keep them as independent historical evidence after that path is removed.
	unit, err := os.ReadFile(filepath.Join("testdata", "retained-unit.service"))
	if err != nil {
		t.Fatal(err)
	}
	unitPath := filepath.Join(root, "etc", "systemd", "system", "ardents-rendezvous-contributor.service")
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath, unit, 0o644); err != nil {
		t.Fatal(err)
	}
	unitDigest := sha256.Sum256(unit)
	record := map[string]any{
		"schema": "ardents-contributor-installation-v1", "profile": manifest.Profile,
		"deployment_id": manifest.DeploymentID, "generation": manifest.Generation,
		"manifest_digest": pin, "installed_files": manifest.Files,
		"systemd_unit_sha256": hex.EncodeToString(unitDigest[:]),
	}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateRoot, "installation.json"), append(recordBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	writeLifecycle(tWriter{root: root}, "READY")
	supervisor.mu.Lock()
	supervisor.active, supervisor.enabled = true, true
	supervisor.mu.Unlock()
}
