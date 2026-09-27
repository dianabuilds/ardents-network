//go:build linux

package forwarding

import (
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"testing"
	"time"
)

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
	server := &closedForwardingServer{dependencies: forwardingDependencies(fixture.config, nil), receiving: &closedForwardingReceivingResources{limits: limits, bootstrap: governor}, clock: fixture.config.now}
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
