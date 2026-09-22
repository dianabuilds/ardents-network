//go:build linux

package node

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedForwardingServeDirectRetainsOuterCleanupFailure(t *testing.T) {
	testClosedForwardingServeDirectOuterAdmission(t, errors.New("outer release failed"))
}

func TestClosedForwardingServeDirectOuterHealthyReleasePreservesPrimary(t *testing.T) {
	testClosedForwardingServeDirectOuterAdmission(t, nil)
}

func testClosedForwardingServeDirectOuterAdmission(t *testing.T, cleanup error) {
	t.Helper()
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := nodeCertificate(t, 247, "forwarding-outer-cleanup-server")
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	host := &cleanupFailureHost{release: cleanup}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: certificate, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	receiver, ok := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, fixture.now)
	if !ok {
		t.Fatal("receiver unavailable")
	}
	token := closedRestrictionToken(t, fixture)
	spends, err := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, receiving: &closedForwardingReceivingResources{spends: spends, limits: limits}, host: host, clock: fixture.config.now}
	var expired atomic.Bool
	host.afterReserve = func() { expired.Store(true) }
	outerClock := func() time.Time {
		if expired.Load() {
			return fixture.now.Add(10 * time.Second)
		}
		return fixture.now
	}
	outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: fixture.now.Add(time.Hour)}, limits, outerClock)
	if err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	result := make(chan error, 1)
	parent := make(chan struct{})
	go func() {
		defer close(parent)
		serveClosedOuter(t.Context(), local, outer, func(ctx context.Context, lane *route.ClosedOuterBridgeLane) {
			secured, err := route.AcceptClosedRoleTLS(ctx, lane, certificate, fixture.now.Add(time.Hour))
			if err == nil {
				err = lane.BeginInnerHello()
			}
			var hello route.ClosedLaneFrame
			if err == nil {
				hello, err = route.ReadClosedLaneFrame(secured)
			}
			if err == nil {
				_, err = route.DecodeClosedHello(hello.Body)
				if err == nil {
					err = lane.Activate(mustClosedForwardingHello(t, receiver, fixture.now.Add(time.Hour)))
				}
			}
			if err == nil {
				err = server.serveDirect(ctx, secured, &hello, [32]byte{}, route.ClosedChildOrdinary, lane)
			}
			result <- err
		})
	}()
	defer func() { _ = peer.Close(); <-parent }()
	hello := mustClosedForwardingHello(t, receiver, fixture.now.Add(time.Hour))
	body, _ := route.EncodeClosedHello(hello)
	if err = route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if _, err = route.ReadClosedLaneFrame(peer); err != nil {
		t.Fatal(err)
	}
	open, _ := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, Deadline: hello.Deadline}, route.ClosedChildOrdinary)
	if err = route.WriteClosedLaneFrame(peer, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: open}); err != nil {
		t.Fatal(err)
	}
	innerRaw := &outerTestInnerConn{outer: peer, lane: 1}
	inner, err := route.OpenClosedRoleTLS(t.Context(), innerRaw, serverKey, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = route.EncodeClosedHello(hello)
	if err = route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if err = route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)}); err != nil {
		t.Fatal(err)
	}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			if _, readErr := route.ReadClosedLaneFrame(peer); readErr != nil {
				return
			}
		}
	}()
	_ = inner.Close()
	err = <-result
	_ = peer.Close()
	<-drained
	if err == nil || !strings.Contains(err.Error(), "outer admission") {
		t.Fatalf("outer refusal = %v", err)
	}
	if cleanup != nil && !errors.Is(err, cleanup) {
		t.Fatalf("outer refusal lost cleanup = %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("outer ownership = %d/%d", host.reserved.Load(), host.released.Load())
	}
}

func TestClosedForwardingServeDirectRetainsConstructorCleanupFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := nodeCertificate(t, 245, "forwarding-constructor-cleanup-server")
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	cleanup := errors.New("host release failed")
	host := &cleanupFailureHost{release: cleanup}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: certificate, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	receiver, available := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, fixture.now)
	if !available {
		t.Fatal("fixture receiver unavailable")
	}
	spends, err := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	clock := func() time.Time {
		// The first eight reads cover receiver/channel validation and the
		// completed admission. The ninth is forwarding construction.
		if calls.Add(1) > 8 {
			return time.Time{}
		}
		return fixture.now
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, receiving: &closedForwardingReceivingResources{spends: spends, limits: limits}, host: host, clock: clock}
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, closedRestrictionToken(t, fixture))
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "channel is invalid") {
		t.Fatalf("constructor refusal lost cleanup or primary: %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("constructor refusal ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	host.release = nil
	calls.Store(0)
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, closedRestrictionToken(t, fixture))
	if err == nil || errors.Is(err, cleanup) || !strings.Contains(err.Error(), "channel is invalid") {
		t.Fatalf("healthy constructor refusal = %v", err)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("healthy constructor ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
}

func TestClosedForwardingServeDirectSuccessfulHandoffLeavesCleanupToForwarding(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := nodeCertificate(t, 246, "forwarding-handoff-cleanup-server")
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	cleanup := errors.New("successor release failed")
	host := &cleanupFailureHost{release: cleanup}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: certificate, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	receiver, available := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, fixture.now)
	if !available {
		t.Fatal("fixture receiver unavailable")
	}
	spends, err := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, receiving: &closedForwardingReceivingResources{spends: spends, limits: limits}, host: host, clock: fixture.config.now}
	client, result := closedForwardingServeDirectAccepted(t, server, serverKey, receiver, closedRestrictionToken(t, fixture))
	joined := false
	t.Cleanup(func() {
		_ = client.Close()
		if !joined {
			<-result
		}
	})
	if host.reserved.Load() != 1 || host.released.Load() != 0 {
		t.Fatalf("live successor ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	if err = client.Close(); err != nil {
		t.Fatal(err)
	}
	err = <-result
	joined = true
	if !errors.Is(err, cleanup) {
		t.Fatalf("successful handoff cleanup = %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("joined successor ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
}

func mustClosedForwardingHello(t *testing.T, receiver route.ClosedRoleReceiver, deadline time.Time) route.ClosedHello {
	t.Helper()
	return route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{77}, Deadline: deadline}
}
