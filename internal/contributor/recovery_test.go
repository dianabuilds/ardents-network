package contributor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestNextCommandRecoversUpdateInterruptedAfterPreviousGenerationWasMoved(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("37", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	installRetainedContributorFixture(t, hostRoot, first, firstPin, supervisor)
	privateRoot := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor")
	installed, err := os.ReadFile(filepath.Join(privateRoot, "installation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateRoot, "update.json"), []byte(`{"schema":"ardents-contributor-updating-v1","previous":`+strings.TrimSpace(string(installed))+"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	programRoot := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor")
	configRoot := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "config")
	if err := os.Rename(filepath.Join(programRoot, "current"), filepath.Join(programRoot, "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(configRoot, "current"), filepath.Join(configRoot, "previous")); err != nil {
		t.Fatal(err)
	}
	startsBefore := supervisor.startCount()
	report, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Generation != 1 || report.Active || report.LifecycleState != "WITHDRAWN" {
		t.Fatalf("recovered report = %+v", report)
	}
	if starts := supervisor.startCount(); starts != startsBefore {
		t.Fatalf("pre-Control recovery started old bytes: calls = %d, want %d", starts, startsBefore)
	}
	for _, path := range []string{filepath.Join(programRoot, "previous"), filepath.Join(configRoot, "previous")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("recovery residue %s remains: %v", path, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(privateRoot, "update.json")); !os.IsNotExist(err) {
		t.Fatalf("recovered update marker remains: %v", err)
	}
}

func TestControlRejectsIncompleteContributorUpdateResidueWithoutTransitionRecord(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		residue string
		paired  bool
	}{
		{name: "next-unpaired", residue: "next"},
		{name: "previous-unpaired", residue: "previous"},
		{name: "next-paired", residue: "next", paired: true},
		{name: "previous-paired", residue: "previous", paired: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			supervisor := &profileSupervisor{hostRoot: root}
			bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5a", 32))
			installRetainedContributorFixture(t, root, bundle, pin, supervisor)
			startsBefore := supervisor.startCount()
			programResidue := filepath.Join(root, "usr", "lib", "ardents-contributor", test.residue)
			var err error
			if err = os.Mkdir(programResidue, 0o755); err != nil {
				t.Fatal(err)
			}
			programSentinel := filepath.Join(programResidue, "foreign-program")
			if err = os.WriteFile(programSentinel, []byte("retain-program"), 0o600); err != nil {
				t.Fatal(err)
			}
			var configSentinel string
			if test.paired {
				configResidue := filepath.Join(root, "var", "lib", "private", "ardents-contributor", "config", test.residue)
				if err = os.Mkdir(configResidue, 0o700); err != nil {
					t.Fatal(err)
				}
				configSentinel = filepath.Join(configResidue, "foreign-config")
				if err = os.WriteFile(configSentinel, []byte("retain-config"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reopened.Control(t.Context(), contributor.Diagnose, ""); err == nil {
				t.Fatalf("Diagnose accepted %s residue without a transition record", test.name)
			}
			if starts := supervisor.startCount(); starts != startsBefore {
				t.Fatalf("ambiguous recovery started old bytes: calls = %d, want %d", starts, startsBefore)
			}
			for path, want := range map[string]string{programSentinel: "retain-program", configSentinel: "retain-config"} {
				if path == "" {
					continue
				}
				raw, readErr := os.ReadFile(path)
				if readErr != nil || string(raw) != want {
					t.Fatalf("ambiguous residue %s = %q, %v; want retained %q", path, raw, readErr, want)
				}
			}
		})
	}
}

func TestControlRecoversIncompleteContributorUpdateResidueWithTransitionRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	supervisor := &profileSupervisor{hostRoot: root}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5b", 32))
	installRetainedContributorFixture(t, root, bundle, pin, supervisor)
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
	startsBefore := supervisor.startCount()
	reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := reopened.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Fatalf("Diagnose with authenticated incomplete residue = %v", err)
	}
	if !report.Active || report.Generation != 1 || report.LifecycleState != "READY" {
		t.Fatalf("recovered active report = %+v", report)
	}
	if starts := supervisor.startCount(); starts != startsBefore {
		t.Fatalf("active pre-Control recovery started old bytes: calls = %d, want %d", starts, startsBefore)
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

func TestControlDoesNotRestartStoppedPredecessorBeforeCleaningUpdateRecord(t *testing.T) {
	root := t.TempDir()
	supervisor := &profileSupervisor{hostRoot: root}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("5c", 32))
	installRetainedContributorFixture(t, root, bundle, pin, supervisor)
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
	startsBefore := supervisor.startCount()
	reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := reopened.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || report.Active || report.LifecycleState != "WITHDRAWN" {
		t.Fatalf("recovered report = %+v, %v", report, err)
	}
	if starts := supervisor.startCount(); starts != startsBefore {
		t.Fatalf("pre-Control recovery started old bytes: calls = %d, want %d", starts, startsBefore)
	}
}

func TestWithdrawRecoversInterruptedUpdateWithoutStartAndRemoveKeepsConfirmation(t *testing.T) {
	root := t.TempDir()
	deployment := strings.Repeat("5e", 32)
	supervisor := &profileSupervisor{hostRoot: root}
	bundle, pin := writeContributorBundle(t, 1, deployment)
	installRetainedContributorFixture(t, root, bundle, pin, supervisor)
	privateRoot := filepath.Join(root, "var", "lib", "private", "ardents-contributor")
	previous, err := os.ReadFile(filepath.Join(privateRoot, "installation.json"))
	if err != nil {
		t.Fatal(err)
	}
	update := []byte(`{"schema":"ardents-contributor-updating-v1","previous":` + strings.TrimSpace(string(previous)) + "}\n")
	if err = os.WriteFile(filepath.Join(privateRoot, "update.json"), update, 0o600); err != nil {
		t.Fatal(err)
	}
	startsBefore := supervisor.startCount()

	reopened, err := contributor.Open(contributor.Config{Root: root, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	withdrawn, err := reopened.Control(t.Context(), contributor.Withdraw, "")
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.DeploymentID != deployment || withdrawn.Generation != 1 || withdrawn.Active || withdrawn.Enabled || withdrawn.LifecycleState != "WITHDRAWN" {
		t.Fatalf("withdrawn report = %+v", withdrawn)
	}
	if starts := supervisor.startCount(); starts != startsBefore {
		t.Fatalf("withdraw recovery started old bytes: calls = %d, want %d", starts, startsBefore)
	}
	if _, err := reopened.Control(t.Context(), contributor.Remove, strings.Repeat("00", 32)); err == nil {
		t.Fatal("remove accepted a foreign deployment confirmation")
	}
	if _, err := os.Stat(filepath.Join(privateRoot, "installation.json")); err != nil {
		t.Fatalf("foreign confirmation changed owned installation: %v", err)
	}
	removed, err := reopened.Control(t.Context(), contributor.Remove, deployment)
	if err != nil || removed.LifecycleState != "REMOVED" || removed.DeploymentID != deployment {
		t.Fatalf("removed report = %+v, %v", removed, err)
	}
	if starts := supervisor.startCount(); starts != startsBefore {
		t.Fatalf("remove recovery started old bytes: calls = %d, want %d", starts, startsBefore)
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
	installRetainedContributorFixture(t, root, bundle, pin, supervisor)
	manager := filepath.Join(root, "usr", "lib", "ardents-contributor", "ardents-node")
	if err := os.WriteFile(manager, []byte("altered-manager"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Control(t.Context(), contributor.Diagnose, ""); err == nil {
		t.Fatal("Diagnose accepted a modified management executable")
	}
}
