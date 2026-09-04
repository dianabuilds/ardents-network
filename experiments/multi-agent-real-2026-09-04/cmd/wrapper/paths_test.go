//go:build ignore

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrepareRunFixesPersonaPathsAndPrivatePlans(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	now := func() time.Time { return time.Date(2026, 9, 4, 1, 2, 3, 0, time.UTC) }
	manifestPath, manifest, err := prepareRun(evidence, now, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RunID != "20260904T010203Z-2a2a2a2a2a2a" {
		t.Fatalf("run id = %q", manifest.RunID)
	}
	if manifestPath != filepath.Join(evidence, "runs", manifest.RunID, "manifest.json") {
		t.Fatalf("manifest path = %q", manifestPath)
	}
	for _, name := range []string{"honest_user", "battery_saver", "probe_consumer"} {
		persona := manifest.Personas[name]
		if strings.Contains(persona.StateRoot, "tick") || strings.Contains(persona.LocalRoleStateRoot, "tick") {
			t.Fatalf("%s has rotating path: %#v", name, persona)
		}
		if persona.StateRoot != "/workspace/evidence/runs/"+manifest.RunID+"/"+name+"/state" {
			t.Fatalf("%s state root = %q", name, persona.StateRoot)
		}
		planPath := filepath.Join(evidence, "runs", manifest.RunID, "plans", name+".json")
		raw, readErr := os.ReadFile(planPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(raw), persona.LocalRoleStateRoot) {
			t.Fatalf("%s plan does not own local role root: %s", name, raw)
		}
	}
	first := refreshArguments(manifest, manifest.Personas["honest_user"])
	second := refreshArguments(manifest, manifest.Personas["honest_user"])
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("refresh arguments drifted: %q then %q", first, second)
	}
}

func TestManifestRejectsPlanDriftAfterPrepare(t *testing.T) {
	t.Parallel()
	evidence := writeTestFixtures(t)
	manifestPath, manifest, err := prepareRun(evidence, time.Now, bytes.NewReader(bytes.Repeat([]byte{0x31}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(manifest.HostRunRoot, "plans", "honest_user.json")
	if err := os.WriteFile(planPath, []byte(`{"schema":"ardents-source-plan-v1","local_role_state_root":"/drifted"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(manifestPath); err == nil {
		t.Fatal("manifest accepted a source plan changed after prepare")
	}
}
