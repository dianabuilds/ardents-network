package forwarding

import (
	"crypto/sha256"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"testing"
	"time"
)

func TestClosedBootstrapRefusesUnsupportedReceiverBeforeReservation(t *testing.T) {
	server := &forwardServer{}
	channel, _, err := server.admitBootstrap(route.ClosedRoleReceiver{Subrole: 3}, [32]byte{}, ardp.Hello{}, 0,
		ardp.Frame{Kind: 3, Body: []byte{2}})
	if err == nil || channel != nil {
		t.Fatal("unsupported receiver allocated bootstrap")
	}
}

func TestClosedBootstrapRecipientRequiresCurrentAdjacentAndExactIssuer(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	if err := bootstrapRecipient(fixture.config.source(), fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now, literalForwardingFixtureEndpoint); err != nil {
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
			if err := bootstrapRecipient(fixture.config.source(), fixture.snapshot, fixture.receiver, fixture.peer, fixture.open, fixture.now, literalForwardingFixtureEndpoint); err == nil {
				t.Fatal("invalid bootstrap recipient admitted")
			}
		})
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
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
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
	server := &forwardServer{dependencies: forwardingDependencies(fixture.config, nil), receiving: &receivingResources{limits: limits, bootstrap: governor}, clock: fixture.config.now}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{80}, Deadline: fixture.now.Add(8 * time.Second)}
	channel, _, err := server.admitBootstrap(receiver, [32]byte{}, hello, 225, ardp.Frame{Kind: 3, Body: []byte{2}})
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
