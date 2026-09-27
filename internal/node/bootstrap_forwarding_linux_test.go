//go:build linux

package node

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/forwarding"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

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
		EpochValidFrom: fixture.now, ValidUntil: profile.NotAfter, Profile: carrier.ClosedRouteProfile, Fresh: true, NodeID: fixture.view.Nodes[1].NodeID,
		RecordGeneration: 12, RecordValidUntil: profile.NotAfter, DeclaredFamily: "bootstrap-interior", CandidateCount: 2}
	for index, role := range []state.ClosedRouteNodeView{fixture.view.Nodes[0], fixture.view.Nodes[2]} {
		fixture.snapshot.Candidates[index] = state.NodeDutyCandidate{NodeID: role.NodeID, RecordDigest: role.RecordDigest, PublicKey: [32]byte{byte(31 + index)},
			FamilyID: [32]byte{byte(41 + index)}, Endpoint: "127.0.0.1:41000", CarrierProfile: string(carrier.ClosedCarrierTCP), ValidFrom: fixture.now,
			ValidUntil: profile.NotAfter, AssignmentNotAfter: profile.NotAfter}
	}
	fixture.config = runtimeConfig{Config: Config{CurrentClosedRoute: func() (state.ClosedRouteView, error) { return fixture.view, nil }}}
	var available bool
	fixture.receiver, available = closedRouteReceiver(fixture.config, fixture.snapshot, ardp.PurposeForwarding, fixture.now)
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
	fixture.snapshot.CarrierProfile = string(carrier.ClosedCarrierTCP)
	fixture.config.NetworkID = fixture.snapshot.NetworkID
	fixture.config.NodeID = fixture.snapshot.NodeID
	fixture.config.IdentityKey = identity
	fixture.config.CheckPlacement = func() error { return nil }
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: t.TempDir(), HostingRoot: t.TempDir(),
		Certificate: tlsCertificate(identity), ConnectionLimit: 1, DrainTimeout: time.Second,
		AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}}
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

func TestClosedBootstrapRecipientRequiresCurrentAdjacentAndExactIssuer(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	if err := forwarding.BootstrapRecipient(nodeAuthority(fixture.config), fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now, literalNodeEndpoint); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*closedBootstrapFixture){
		"direct interior":       func(f *closedBootstrapFixture) { f.peer = [32]byte{} },
		"foreign adjacency":     func(f *closedBootstrapFixture) { f.peer[0]++ },
		"wrong adjacent domain": func(f *closedBootstrapFixture) { f.view.Nodes[0].RoleDomain = 3 },
		"wrong adjacent record": func(f *closedBootstrapFixture) { f.view.Nodes[0].RecordDigest[0]++ },
		"same adjacent family": func(f *closedBootstrapFixture) {
			f.snapshot.Candidates[0].FamilyID = sha256.Sum256([]byte(f.snapshot.DeclaredFamily))
		},
		"same issuer family": func(f *closedBootstrapFixture) { f.snapshot.Candidates[1].FamilyID = f.snapshot.Candidates[0].FamilyID },
		"future adjacent":    func(f *closedBootstrapFixture) { f.snapshot.Candidates[0].ValidFrom = f.now.Add(time.Second) },
		"expired adjacent":   func(f *closedBootstrapFixture) { f.snapshot.Candidates[0].ValidUntil = f.now },
		"old profile":        func(f *closedBootstrapFixture) { f.view.Profile.Digest[0]++ },
		"wrong issuer":       func(f *closedBootstrapFixture) { f.view.Profile.IssuerNodeID[0]++ },
		"wrong issuer duty":  func(f *closedBootstrapFixture) { f.open.NextDutyGeneration++ },
		"private purpose": func(f *closedBootstrapFixture) {
			f.open.Purpose = ardp.PurposeDataJoin
			f.view.Nodes[2].Subrole = 4
		},
		"arbitrary endpoint": func(f *closedBootstrapFixture) { f.snapshot.Candidates[1].Endpoint = "example.invalid:443" },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newClosedBootstrapFixture(t)
			mutate(fixture)
			if err := forwarding.BootstrapRecipient(nodeAuthority(fixture.config), fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now, literalNodeEndpoint); err == nil {
				t.Fatal("invalid bootstrap recipient admitted")
			}
		})
	}
}
