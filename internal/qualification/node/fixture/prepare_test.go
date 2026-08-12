package fixture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/node/fixture"
)

func TestPrepareCreatesBoundedIsolatedFixtureAsCurrentUser(t *testing.T) {
	root := filepath.Join(t.TempDir(), "node")
	err := fixture.Prepare(fixture.PrepareConfig{Root: root, Now: time.Unix(1_800_000_100, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || len(raw) > 64<<10 {
		t.Fatalf("manifest read/size: %v/%d", err, len(raw))
	}
	var manifest struct {
		Schema string   `json:"schema"`
		Zones  []string `json:"zones"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != "ardents-h3-node-manifest-v1" || len(manifest.Zones) != 5 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, path := range []string{
		"artifacts/epoch-0001.bin", "artifacts/epoch-0002.bin", "artifacts/inputs/0000.bin",
		"artifacts/material-0001-0000.bin", "artifacts/material-0002-0001.bin",
		"plans/source-1.json", "plans/source-2.json", "plans/node-1.json", "plans/node-1-emfile.json", "plans/node-2.json", "plans/endpoint.json",
		"state/e", "state/s1", "state/s2", "state/n1", "state/n2",
		"secrets/e", "secrets/s1", "secrets/s2", "secrets/n1", "secrets/n2", "secrets/h",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Errorf("fixture path %s: %v", path, err)
		}
	}
	if err := fixture.Prepare(fixture.PrepareConfig{Root: root, Now: time.Now()}); err == nil {
		t.Fatal("non-empty fixture root was reused")
	}
}

func TestValidateRejectsCorruptedManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "node")
	if err := fixture.Prepare(fixture.PrepareConfig{Root: root, Now: time.Unix(1_800_000_100, 0).UTC()}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifest, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Validate(root); err == nil {
		t.Fatal("corrupted manifest was accepted")
	}
}
