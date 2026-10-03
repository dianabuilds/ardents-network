package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
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

type closedBootstrapFixture struct {
	now      time.Time
	config   runtimeConfig
	snapshot state.NodeDuty
	receiver route.ClosedRoleReceiver
	peer     [32]byte
	open     route.ClosedOpen
	view     state.ClosedRouteView
}

// Projection fixture only: accepted State parsing and its signature/record
// joins have their own tests. This exercises the Node's additional pre-dial
// adjacency, role transition and family exclusions.
func newClosedBootstrapFixture(t *testing.T) *closedBootstrapFixture {
	t.Helper()
	fixture := &closedBootstrapFixture{now: time.Unix(1_800_000_000, 0).UTC()}
	generation := [32]byte{1}
	profile := state.ClosedProfileView{NetworkID: [32]byte{2}, StateGeneration: generation, StateDigest: [32]byte{3}, Digest: [32]byte{4},
		Epoch: 5, IssuerNodeID: [32]byte{13}, IssuerDutyGeneration: 13, NotBefore: fixture.now, NotAfter: fixture.now.Add(time.Hour)}
	fixture.view = state.ClosedRouteView{Profile: profile, NodeCount: 3}
	for index := 0; index < 3; index++ {
		fixture.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: [32]byte{byte(11 + index)}, RecordDigest: [32]byte{byte(21 + index)},
			RoleDomain: 1, Subrole: uint8(index + 1), DutyGeneration: uint64(11 + index)}
	}
	fixture.view.Nodes[2].RoleDomain, fixture.view.Nodes[2].Subrole = 2, 6
	fixture.snapshot = state.NodeDuty{Generation: hex.EncodeToString(generation[:]), NetworkID: profile.NetworkID, Epoch: profile.Epoch, Digest: profile.StateDigest,
		EpochValidFrom: fixture.now, ValidUntil: profile.NotAfter, Profile: routecarrier.ClosedRouteProfile, Fresh: true, NodeID: fixture.view.Nodes[1].NodeID,
		RecordGeneration: 12, RecordValidUntil: profile.NotAfter, DeclaredFamily: "bootstrap-interior", CandidateCount: 2}
	for index, role := range []state.ClosedRouteNodeView{fixture.view.Nodes[0], fixture.view.Nodes[2]} {
		fixture.snapshot.Candidates[index] = state.NodeDutyCandidate{NodeID: role.NodeID, RecordDigest: role.RecordDigest, PublicKey: [32]byte{byte(31 + index)},
			FamilyID: [32]byte{byte(41 + index)}, Endpoint: "127.0.0.1:41000", CarrierProfile: string(routecarrier.ClosedCarrierTCP), ValidFrom: fixture.now,
			ValidUntil: profile.NotAfter, AssignmentNotAfter: profile.NotAfter}
	}
	fixture.config = runtimeConfig{Config: Config{CurrentClosedRoute: func() (state.ClosedRouteView, error) { return fixture.view, nil }}}
	var available bool
	fixture.receiver, available = nodeAuthority(fixture.config).Receiver(fixture.snapshot, ardp.PurposeForwarding, fixture.now)
	if !available {
		t.Fatal("fixture receiver unavailable")
	}
	fixture.peer = fixture.snapshot.Candidates[0].PublicKey
	fixture.open = route.ClosedOpen{NextNodeID: profile.IssuerNodeID, NextDutyGeneration: profile.IssuerDutyGeneration,
		Purpose: ardp.PurposeIssuer, Deadline: fixture.now.Add(time.Second)}
	return fixture
}

func TestClosedAdmissionUsesOneConsistentRouteProjection(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	identity := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	copy(fixture.snapshot.NodePublicKey[:], identity.Public().(ed25519.PublicKey))
	fixture.snapshot.RecordPresent = true
	fixture.snapshot.RecordValidFrom = fixture.now
	fixture.snapshot.ProbeEndpoint = "127.0.0.1:41000"
	fixture.snapshot.CarrierProfile = string(routecarrier.ClosedCarrierTCP)
	fixture.config.NetworkID = fixture.snapshot.NetworkID
	fixture.config.NodeID = fixture.snapshot.NodeID
	fixture.config.IdentityKey = identity
	fixture.config.CheckPlacement = func() error { return nil }
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: t.TempDir(), HostingRoot: t.TempDir(),
		Certificate: tlsCertificate(identity), ConnectionLimit: 1, DrainTimeout: time.Second,
		AdmissionTraffic: hostingbudget.Traffic{Tx: 1}, TerminationTraffic: hostingbudget.Traffic{Tx: 1}}
	calls := 0
	fixture.config.CurrentClosedRoute = func() (state.ClosedRouteView, error) {
		calls++
		if calls > 1 {
			return state.ClosedRouteView{}, errors.New("second Route projection read")
		}
		return fixture.view, nil
	}
	if got := assessAdmission(fixture.config, fixture.snapshot); got.kind != admissionReady || calls != 1 {
		t.Fatalf("closed admission did not retain one Route projection: %+v calls=%d", got, calls)
	}
}

func tlsCertificate(identity ed25519.PrivateKey) tls.Certificate {
	return tls.Certificate{PrivateKey: identity}
}
