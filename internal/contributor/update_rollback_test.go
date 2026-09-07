package contributor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestFailedSuccessorRestoresPreviousReadyGeneration(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("36", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	supervisor.failNextStart = true
	second, secondPin := writeContributorBundle(t, 2, deployment)
	if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
		t.Fatal("failed successor was reported as installed")
	}
	report, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || report.Generation != 1 || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("rolled-back report = %+v, %v", report, err)
	}
	program := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "current", "ardents-node")
	raw, err := os.ReadFile(program)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
		t.Fatalf("rolled-back program = %q, %v", raw, err)
	}
	manager := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "ardents-node")
	raw, err = os.ReadFile(manager)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
		t.Fatalf("rolled-back management program = %q, %v", raw, err)
	}
}

func TestSuccessorBackfillsMissingManagementExecutable(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("37", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	manager := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "ardents-node")
	if err := os.Remove(manager); err != nil {
		t.Fatal(err)
	}
	second, secondPin := writeContributorBundle(t, 2, deployment)
	if _, err := profile.Apply(t.Context(), second, secondPin); err != nil {
		t.Fatalf("successor did not backfill a missing manager: %v", err)
	}
	raw, err := os.ReadFile(manager)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v2" {
		t.Fatalf("backfilled management program = %q, %v", raw, err)
	}
}

func TestFailedManagerRollbackRetainsUpdateRecordForRecovery(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("38", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	manager := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "ardents-node")
	managerTemporary := manager + ".new"
	supervisor.beforeNextStart = func() {
		if err := os.Mkdir(managerTemporary, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(managerTemporary, "blocked"), []byte("blocked"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	supervisor.failNextStart = true
	second, secondPin := writeContributorBundle(t, 2, deployment)
	if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
		t.Fatal("failed successor was reported as installed")
	}
	updateRecord := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "update.json")
	if _, err := os.Lstat(updateRecord); err != nil {
		t.Fatalf("update record was removed after manager rollback failure: %v", err)
	}
	if err := os.RemoveAll(managerTemporary); err != nil {
		t.Fatal(err)
	}
	reopened, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Control(t.Context(), contributor.Diagnose, ""); err != nil {
		t.Fatalf("recovery after manager rollback failure = %v", err)
	}
	if _, err := os.Lstat(updateRecord); !os.IsNotExist(err) {
		t.Fatalf("recovered update record remains: %v", err)
	}
	raw, err := os.ReadFile(manager)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
		t.Fatalf("recovered management program = %q, %v", raw, err)
	}
}

func TestFailedSuccessorRestoresManagerFromAuthenticatedCurrentGeneration(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, manager string){
		"missing manager": func(t *testing.T, manager string) {
			t.Helper()
			if err := os.Remove(manager); err != nil {
				t.Fatal(err)
			}
		},
		"modified manager": func(t *testing.T, manager string) {
			t.Helper()
			if err := os.WriteFile(manager, []byte("modified-manager"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			hostRoot := t.TempDir()
			deployment := strings.Repeat("3a", 32)
			supervisor := &profileSupervisor{hostRoot: hostRoot}
			profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			first, firstPin := writeContributorBundle(t, 1, deployment)
			if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
				t.Fatal(err)
			}
			manager := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "ardents-node")
			prepare(t, manager)
			supervisor.failNextStart = true
			second, secondPin := writeContributorBundle(t, 2, deployment)
			if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
				t.Fatal("failed successor was reported as installed")
			}
			raw, err := os.ReadFile(manager)
			if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
				t.Fatalf("rolled-back management program = %q, %v", raw, err)
			}
		})
	}
}

func TestFailedManagerReplacementRetainsUpdateRecordForRecovery(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, manager string){
		"missing manager": func(t *testing.T, manager string) {
			t.Helper()
			if err := os.Remove(manager); err != nil {
				t.Fatal(err)
			}
		},
		"modified manager": func(t *testing.T, manager string) {
			t.Helper()
			if err := os.WriteFile(manager, []byte("modified-manager"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			hostRoot := t.TempDir()
			deployment := strings.Repeat("3b", 32)
			supervisor := &profileSupervisor{hostRoot: hostRoot}
			profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			first, firstPin := writeContributorBundle(t, 1, deployment)
			if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
				t.Fatal(err)
			}
			manager := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "ardents-node")
			prepare(t, manager)
			temporary := manager + ".new"
			if err := os.Mkdir(temporary, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(temporary, "blocked"), []byte("blocked"), 0o600); err != nil {
				t.Fatal(err)
			}
			second, secondPin := writeContributorBundle(t, 2, deployment)
			if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
				t.Fatal("failed manager replacement was reported as installed")
			}
			updateRecord := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "update.json")
			if _, err := os.Lstat(updateRecord); err != nil {
				t.Fatalf("update record was removed after manager replacement failure: %v", err)
			}
			if err := os.RemoveAll(temporary); err != nil {
				t.Fatal(err)
			}
			reopened, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.Control(t.Context(), contributor.Diagnose, ""); err != nil {
				t.Fatalf("recovery after manager replacement failure = %v", err)
			}
			raw, err := os.ReadFile(manager)
			if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
				t.Fatalf("recovered management program = %q, %v", raw, err)
			}
		})
	}
}
