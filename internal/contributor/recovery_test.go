package contributor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestControlRejectsIncompleteContributorUpdateResidueWithoutTransitionRecord(t *testing.T) {
	t.Parallel()
	for _, residue := range []string{"next", "previous"} {
		t.Run(residue, func(t *testing.T) {
			root := t.TempDir()
			supervisor := &profileSupervisor{hostRoot: root}
			profile, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5a", 32))
			if _, err = profile.Apply(t.Context(), bundle, pin); err != nil {
				t.Fatal(err)
			}
			programResidue := filepath.Join(root, "usr", "lib", "ardents-contributor", residue)
			if err = os.Mkdir(programResidue, 0o755); err != nil {
				t.Fatal(err)
			}
			reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reopened.Control(t.Context(), contributor.Diagnose, ""); err == nil {
				t.Fatalf("Diagnose accepted incomplete %s residue without a transition record", residue)
			}
		})
	}
}

func TestControlRecoversIncompleteContributorUpdateResidueWithTransitionRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	supervisor := &profileSupervisor{hostRoot: root}
	profile, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5b", 32))
	if _, err = profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(root, "var", "lib", "private", "ardents-contributor")
	previous, err := os.ReadFile(filepath.Join(privateRoot, "installation.json"))
	if err != nil {
		t.Fatal(err)
	}
	update := []byte(`{"schema":"ardents-contributor-updating-v1","previous":` + strings.TrimSpace(string(previous)) + "}\n")
	if err = os.WriteFile(filepath.Join(privateRoot, "update.json"), update, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := filepath.Join(root, "usr", "lib", "ardents-contributor", "ardents-node")
	expectedManager, err := os.ReadFile(filepath.Join(root, "usr", "lib", "ardents-contributor", "current", "ardents-node"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manager, []byte("uncommitted-successor"), 0o755); err != nil {
		t.Fatal(err)
	}
	residue := filepath.Join(root, "usr", "lib", "ardents-contributor", "next")
	if err = os.Mkdir(residue, 0o755); err != nil {
		t.Fatal(err)
	}
	reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Control(t.Context(), contributor.Diagnose, ""); err != nil {
		t.Fatalf("Diagnose with authenticated incomplete residue = %v", err)
	}
	if managerRaw, readErr := os.ReadFile(manager); readErr != nil || string(managerRaw) != string(expectedManager) {
		t.Fatalf("recovered management program = %q, %v", managerRaw, readErr)
	}
	for _, path := range []string{residue, filepath.Join(privateRoot, "update.json")} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("recovered residue %s remains: %v", path, err)
		}
	}
}

func TestControlRestartsStoppedPredecessorBeforeCleaningUpdateRecord(t *testing.T) {
	root := t.TempDir()
	supervisor := &profileSupervisor{hostRoot: root}
	profile, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5c", 32))
	if _, err = profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	privateRoot := filepath.Join(root, "var", "lib", "private", "ardents-contributor")
	previous, err := os.ReadFile(filepath.Join(privateRoot, "installation.json"))
	if err != nil {
		t.Fatal(err)
	}
	update := []byte(`{"schema":"ardents-contributor-updating-v1","previous":` + strings.TrimSpace(string(previous)) + "}\n")
	if err = os.WriteFile(filepath.Join(privateRoot, "update.json"), update, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = supervisor.Do(t.Context(), contributor.SupervisorStop); err != nil {
		t.Fatal(err)
	}
	reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := reopened.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("recovered report = %+v, %v", report, err)
	}
}

func TestDiagnoseRejectsModifiedManagementExecutable(t *testing.T) {
	root := t.TempDir()
	supervisor := &profileSupervisor{hostRoot: root}
	profile, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5d", 32))
	if _, err := profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	manager := filepath.Join(root, "usr", "lib", "ardents-contributor", "ardents-node")
	if err := os.WriteFile(manager, []byte("altered-manager"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Control(t.Context(), contributor.Diagnose, ""); err == nil {
		t.Fatal("Diagnose accepted a modified management executable")
	}
}
