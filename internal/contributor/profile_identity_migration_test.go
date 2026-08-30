package contributor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

const (
	canonicalRendezvousProfile = "ardents-rendezvous-dedicated-host-v1"
	legacyRendezvousProfile    = "h4-5-rendezvous-alpha-v1"
)

func TestLegacyBundleInstallsAndReopensThroughInterruptedUpdateRecovery(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("41", 32)
	bundle, pin := writeContributorBundleProfiles(t, 1, deployment, legacyRendezvousProfile, legacyRendezvousProfile)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := profile.Apply(t.Context(), bundle, pin)
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != canonicalRendezvousProfile {
		t.Fatalf("legacy bundle install report profile = %q", report.Profile)
	}
	recordPath := contributorRecordPath(hostRoot)
	if got := persistedContributorProfile(t, recordPath); got != canonicalRendezvousProfile {
		t.Fatalf("legacy bundle installation record profile = %q", got)
	}

	writePersistedContributorProfile(t, recordPath, legacyRendezvousProfile)
	programNext := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "next")
	configNext := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "config", "next")
	for _, path := range []string{programNext, configNext} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Profile != canonicalRendezvousProfile || recovered.Generation != 1 {
		t.Fatalf("reopened legacy installation report = %+v", recovered)
	}
	for _, path := range []string{programNext, configNext} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("recovered update residue %s remains: %v", path, err)
		}
	}
}

func TestLegacyInstallationRecordAdvancesToCanonicalSuccessor(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("42", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundleProfiles(t, 1, deployment, legacyRendezvousProfile, legacyRendezvousProfile)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	recordPath := contributorRecordPath(hostRoot)
	writePersistedContributorProfile(t, recordPath, legacyRendezvousProfile)

	successor, successorPin := writeContributorBundle(t, 2, deployment)
	report, err := profile.Apply(t.Context(), successor, successorPin)
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != canonicalRendezvousProfile || report.Generation != 2 {
		t.Fatalf("canonical successor report = %+v", report)
	}
	if got := persistedContributorProfile(t, recordPath); got != canonicalRendezvousProfile {
		t.Fatalf("successor installation record profile = %q", got)
	}
}

func TestContributorProfileReadersRefuseUnknownIdentity(t *testing.T) {
	t.Run("bundle manifest", func(t *testing.T) {
		bundle, pin := writeContributorBundleProfiles(t, 1, strings.Repeat("43", 32), "unknown-profile-v1", canonicalRendezvousProfile)
		profile, err := contributor.Open(contributor.Config{Root: t.TempDir(), Supervisor: &profileSupervisor{}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := profile.Apply(t.Context(), bundle, pin); err == nil {
			t.Fatal("unknown Contributor bundle profile was accepted")
		}
	})

	t.Run("bundle Node plan", func(t *testing.T) {
		bundle, pin := writeContributorBundleProfiles(t, 1, strings.Repeat("44", 32), canonicalRendezvousProfile, "unknown-profile-v1")
		profile, err := contributor.Open(contributor.Config{Root: t.TempDir(), Supervisor: &profileSupervisor{}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := profile.Apply(t.Context(), bundle, pin); err == nil {
			t.Fatal("unknown Contributor Node plan profile was accepted")
		}
	})

	t.Run("installation record", func(t *testing.T) {
		hostRoot := t.TempDir()
		supervisor := &profileSupervisor{hostRoot: hostRoot}
		profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
		if err != nil {
			t.Fatal(err)
		}
		bundle, pin := writeContributorBundle(t, 1, strings.Repeat("45", 32))
		if _, err := profile.Apply(t.Context(), bundle, pin); err != nil {
			t.Fatal(err)
		}
		writePersistedContributorProfile(t, contributorRecordPath(hostRoot), "unknown-profile-v1")
		if _, err := profile.Control(t.Context(), contributor.Diagnose, ""); err == nil {
			t.Fatal("unknown Contributor installation record profile was accepted")
		}
	})
}

func contributorRecordPath(hostRoot string) string {
	return filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "installation.json")
}

func persistedContributorProfile(t *testing.T, path string) string {
	t.Helper()
	var record map[string]any
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	profile, _ := record["profile"].(string)
	return profile
}

func writePersistedContributorProfile(t *testing.T, path, profile string) {
	t.Helper()
	var record map[string]any
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	record["profile"] = profile
	raw, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
