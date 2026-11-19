package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The ACA1 inspection and qualification-preparation routes are retired
// (ADR-0127, #647). Each refuses at dispatch before parsing flags, reading a
// plan or enrollment document, or creating an output root. Nonexistent and
// hostile existing inputs reach the identical refusal; retained floor bytes
// and supplied files stay unchanged.
func TestRetiredAlphaControlRoutesRefuseBeforeEffects(t *testing.T) {
	for _, route := range []struct{ name, refusal string }{
		{"inspect-bundle", "inspect-bundle is retired"},
		{"inspect-transitions", "inspect-transitions is retired"},
		{"prepare-qualification-evidence", "prepare-qualification-evidence is retired"},
		{"prepare-qualification-catalog", "prepare-qualification-catalog is retired"},
		{"prepare-qualification-node-record", "prepare-qualification-node-record is retired; use ardents-next network prepare-initial-node-record"},
		{"prepare-qualification-epoch", "prepare-qualification-epoch is retired; use ardents-next network prepare-initial-epoch"},
	} {
		t.Run(route.name, func(t *testing.T) {
			directory := t.TempDir()
			hostilePlan := filepath.Join(directory, "plan.json")
			if err := os.WriteFile(hostilePlan, []byte(`{"schema":"hostile"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			hostileEnrollment := filepath.Join(directory, "alpha-enrollment.json")
			if err := os.WriteFile(hostileEnrollment, []byte(`{"schema":"ardents-alpha-enrollment-input-v1"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			retainedRoot := filepath.Join(directory, "retained")
			if err := os.Mkdir(retainedRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			floorPath := filepath.Join(retainedRoot, "floor.bin")
			if err := os.WriteFile(floorPath, []byte("retained floor"), 0o600); err != nil {
				t.Fatal(err)
			}
			absentOutput := filepath.Join(directory, "absent-output")
			for _, arguments := range [][]string{
				{route.name},
				{route.name, "--enrollment", hostileEnrollment, "--artifact", filepath.Join(directory, "absent-artifact"), "--state-root", retainedRoot, "--at", "2026-11-19T00:00:00Z"},
				{route.name, "--enrollment", filepath.Join(directory, "absent.json"), "--artifact", filepath.Join(directory, "absent-artifact"), "--state-root", absentOutput, "--at", "2026-11-19T00:00:00Z"},
				{route.name, "--plan", hostilePlan, "--output-root", absentOutput},
				{route.name, "--plan", filepath.Join(directory, "absent-plan.json"), "--output-root", retainedRoot},
			} {
				var output bytes.Buffer
				err := run(arguments, &output)
				if err == nil || err.Error() != route.refusal {
					t.Fatalf("retired %s error = %v", route.name, err)
				}
				if output.Len() != 0 {
					t.Fatalf("retired %s output = %q", route.name, output.String())
				}
			}
			if _, err := os.Lstat(absentOutput); !os.IsNotExist(err) {
				t.Fatalf("retired %s created an output root: %v", route.name, err)
			}
			for path, retained := range map[string]string{
				floorPath:         "retained floor",
				hostilePlan:       `{"schema":"hostile"}`,
				hostileEnrollment: `{"schema":"ardents-alpha-enrollment-input-v1"}`,
			} {
				content, err := os.ReadFile(path)
				if err != nil || string(content) != retained {
					t.Fatalf("retired %s changed %s: %q, %v", route.name, path, content, err)
				}
			}
			entries, err := os.ReadDir(retainedRoot)
			if err != nil || len(entries) != 1 {
				t.Fatalf("retired %s changed the retained root contents: %+v, %v", route.name, entries, err)
			}
		})
	}
}
