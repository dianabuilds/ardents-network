package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestResolvePinsLocalRoleRootAcrossWorkingDirectoryChange(t *testing.T) {
	fixture := newLifecycleFixture(t)
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	roleRoot, err := filepath.Abs(fixture.config.LocalRoleStateRoot)
	if err != nil {
		t.Fatal(err)
	}
	relativeRoot, err := filepath.Rel(workingDirectory, roleRoot)
	if err != nil {
		t.Fatal(err)
	}
	fixture.config.LocalRoleStateRoot = relativeRoot
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(context.Context, Event) error { return nil }
	config, err := resolveConfig(fixture.config)
	if err != nil || config.LocalRoleStateRoot != roleRoot {
		t.Fatalf("resolved local role root = %q, %v, want %q", config.LocalRoleStateRoot, err, roleRoot)
	}
	t.Chdir(t.TempDir())
	if err := retainLocalDuty(config, fixture.snapshot, "live"); err != nil {
		t.Fatal(err)
	}
	roles, err := localroles.Open(localroles.Config{Root: roleRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, checkErr := roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily)))
	closeErr := roles.Close()
	if checkErr != nil || closeErr != nil || !conflict {
		t.Fatalf("role was redirected after cwd change: conflict %v, check %v, close %v", conflict, checkErr, closeErr)
	}
	if err := releaseLocalDuty(config); err != nil {
		t.Fatal(err)
	}
	roles, err = localroles.Open(localroles.Config{Root: roleRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, checkErr = roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily)))
	closeErr = roles.Close()
	if checkErr != nil || closeErr != nil || conflict {
		t.Fatalf("role removal was redirected after cwd change: conflict %v, check %v, close %v", conflict, checkErr, closeErr)
	}
}

func TestAdmissionRequiresEveryPrerequisite(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_000_000, 0).UTC()
	config := runtimeConfig{Config: Config{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, IdentityKey: private,
		Probe:          ProbeConfig{ListenAddress: "127.0.0.1:4101", MaximumDuty: time.Second},
		CheckPlacement: func() error { return nil }}, now: func() time.Time { return now }}
	snapshot := state.NodeDuty{NetworkID: config.NetworkID, NodeID: config.NodeID, RecordPresent: true,
		EpochValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), RecordValidFrom: now.Add(-time.Hour),
		RecordValidUntil: now.Add(time.Hour), Profile: "h3-role-probe-v1", Assignment: "domain-a",
		ProbeEndpoint: config.Probe.ListenAddress, ProbeCapacity: 1, Fresh: true}
	copy(snapshot.NodePublicKey[:], public)
	if got := assessAdmission(config, snapshot); got.kind != admissionReady {
		t.Fatalf("complete admission = %+v", got)
	}
	tests := []struct {
		name string
		edit func(*state.NodeDuty, *runtimeConfig)
		want admissionKind
	}{
		{"record", func(s *state.NodeDuty, _ *runtimeConfig) { s.RecordPresent = false }, admissionAbsent},
		{"identity", func(s *state.NodeDuty, _ *runtimeConfig) { s.NodeID[0]++ }, admissionAbsent},
		{"key", func(s *state.NodeDuty, _ *runtimeConfig) { s.NodePublicKey[0]++ }, admissionFailed},
		{"network", func(s *state.NodeDuty, _ *runtimeConfig) { s.NetworkID[0]++ }, admissionFailed},
		{"profile", func(s *state.NodeDuty, _ *runtimeConfig) { s.Profile = "other" }, admissionPrepared},
		{"assignment", func(s *state.NodeDuty, _ *runtimeConfig) { s.Assignment = "" }, admissionPrepared},
		{"capacity", func(s *state.NodeDuty, _ *runtimeConfig) { s.ProbeCapacity = 0 }, admissionPrepared},
		{"endpoint", func(s *state.NodeDuty, _ *runtimeConfig) { s.ProbeEndpoint = "127.0.0.1:9" }, admissionFailed},
		{"conflict", func(s *state.NodeDuty, _ *runtimeConfig) { s.Conflicting = true }, admissionPrepared},
		{"freshness", func(s *state.NodeDuty, _ *runtimeConfig) { s.Fresh = false }, admissionPrepared},
		{"epoch boundary", func(s *state.NodeDuty, _ *runtimeConfig) { s.ValidUntil = now.Add(time.Second) }, admissionPrepared},
		{"record boundary", func(s *state.NodeDuty, _ *runtimeConfig) { s.RecordValidUntil = now.Add(time.Second) }, admissionPrepared},
		{"placement", func(_ *state.NodeDuty, c *runtimeConfig) {
			c.CheckPlacement = func() error { return errors.New("pressure") }
		}, admissionPrepared},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate, candidateConfig := snapshot, config
			test.edit(&candidate, &candidateConfig)
			if got := assessAdmission(candidateConfig, candidate); got.kind != test.want {
				t.Fatalf("admission = %+v, want %v", got, test.want)
			}
		})
	}
}
