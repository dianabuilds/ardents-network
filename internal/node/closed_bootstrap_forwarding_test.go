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
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type closedBootstrapFixture struct {
	now      time.Time
	config   runtimeConfig
	snapshot dutyFacts
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
	fixture.snapshot = dutyFacts{Generation: hex.EncodeToString(generation[:]), NetworkID: profile.NetworkID, Epoch: profile.Epoch, Digest: profile.StateDigest,
		EpochValidFrom: fixture.now, ValidUntil: profile.NotAfter, Profile: route.ClosedRouteProfile, Fresh: true, NodeID: fixture.view.Nodes[1].NodeID,
		RecordGeneration: 12, RecordValidUntil: profile.NotAfter, DeclaredFamily: "bootstrap-interior", CandidateCount: 2}
	for index, role := range []state.ClosedRouteNodeView{fixture.view.Nodes[0], fixture.view.Nodes[2]} {
		fixture.snapshot.Candidates[index] = dutyCandidate{NodeID: role.NodeID, RecordDigest: role.RecordDigest, PublicKey: [32]byte{byte(31 + index)},
			FamilyID: [32]byte{byte(41 + index)}, Endpoint: "127.0.0.1:41000", CarrierProfile: string(route.ClosedCarrierTCP), ValidFrom: fixture.now,
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
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
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
	if err := closedBootstrapRecipient(fixture.config, fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now); err != nil {
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
			if err := closedBootstrapRecipient(fixture.config, fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now); err == nil {
				t.Fatal("invalid bootstrap recipient admitted")
			}
		})
	}
}

func TestClosedBootstrapRefusesUnsupportedReceiverBeforeReservation(t *testing.T) {
	server := &closedForwardingServer{}
	channel, _, err := server.admitBootstrap(route.ClosedRoleReceiver{Subrole: 3}, [32]byte{}, ardp.Hello{}, 0,
		ardp.Frame{Kind: 3, Body: ardp.EncodeBootstrap(true)})
	if err == nil || channel != nil {
		t.Fatal("unsupported receiver allocated bootstrap")
	}
}

func TestClosedBootstrapEntryExportsOnlyRestrictedInteriorChild(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	fixture.snapshot.NodeID = fixture.view.Nodes[0].NodeID
	fixture.snapshot.RecordGeneration = fixture.view.Nodes[0].DutyGeneration
	fixture.snapshot.DeclaredFamily = "entry-bootstrap"
	fixture.snapshot.Candidates[0].NodeID = fixture.view.Nodes[1].NodeID
	fixture.snapshot.Candidates[0].RecordDigest = fixture.view.Nodes[1].RecordDigest
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	receiver, ok := closedRouteReceiver(fixture.config, fixture.snapshot, ardp.PurposeForwarding, fixture.now)
	if !ok {
		t.Fatal("Entry fixture unavailable")
	}
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	governor, err := route.NewClosedBootstrapController(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{config: fixture.config, receiving: &closedForwardingReceivingResources{limits: limits, bootstrap: governor}, clock: fixture.config.now}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{80}, Deadline: fixture.now.Add(8 * time.Second)}
	channel, _, err := server.admitBootstrap(receiver, [32]byte{}, hello, 225, ardp.Frame{Kind: 3, Body: ardp.EncodeBootstrap(true)})
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Cancel()
	next := fixture.view.Nodes[1]
	body, err := route.EncodeClosedOpen(route.ClosedOpen{NextNodeID: next.NodeID, NextDutyGeneration: next.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: hello.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	event, ok := channel.NextAvailable(nil)
	if !ok || event.Restriction != route.ClosedChildIssuerBootstrap {
		t.Fatalf("Entry did not propagate actual bootstrap reservation: %+v", event)
	}
	if _, err := channel.Accept(ardp.Frame{Kind: 2, Body: make([]byte, 355)}); err == nil {
		t.Fatal("Entry bootstrap relabelled after ADMIT")
	}
}
