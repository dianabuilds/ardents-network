//go:build linux

package node

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type cleanupFailureHost struct {
	release      error
	reserved     atomic.Int32
	released     atomic.Int32
	afterReserve func()
}

func (host *cleanupFailureHost) Observe(context.Context) (resource.HostingObservation, error) {
	return resource.HostingObservation{}, nil
}
func (host *cleanupFailureHost) Close() error { return nil }
func (host *cleanupFailureHost) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (closedForwardingHostReservation, error) {
	host.reserved.Add(1)
	if host.afterReserve != nil {
		host.afterReserve()
	}
	return cleanupFailureReservation{host: host}, nil
}

type cleanupFailureReservation struct{ host *cleanupFailureHost }

func (reservation cleanupFailureReservation) Release(context.Context) error {
	reservation.host.released.Add(1)
	return reservation.host.release
}

func TestClosedForwardingServeDirectRetainsDuplicateSpendCleanupFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := rendezvousCertificate(t, 241, "forwarding-cleanup-server")
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	host := &cleanupFailureHost{}
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
	var verificationTicks atomic.Int32
	limits, err := route.NewClosedDutyLimits(func() time.Time { return fixture.now.Add(time.Duration(verificationTicks.Add(1)/100) * time.Second) })
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, spends: spends, limits: limits, host: host, clock: fixture.config.now}
	token := closedRestrictionToken(t, fixture)
	if err := closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token); err != nil {
		t.Fatalf("first admission = %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("first admission ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	cleanup := errors.New("host release failed")
	host.release = cleanup
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token)
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "token") {
		t.Fatalf("duplicate refusal lost cleanup or primary: %v", err)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("failed duplicate ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	host.release = nil
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token)
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("healthy duplicate refusal = %v", err)
	}
	if host.reserved.Load() != 3 || host.released.Load() != 3 {
		t.Fatalf("healthy duplicate ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
}

func TestClosedForwardingServeDirectRetainsExpiredLeaseCleanupFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := rendezvousCertificate(t, 242, "forwarding-expiry-cleanup-server")
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
	leaseDeadline := fixture.now.Add(time.Second)
	var calls atomic.Int32
	clock := func() time.Time {
		if calls.Add(1) > 6 {
			return leaseDeadline
		}
		return fixture.now
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, spends: spends, limits: limits, host: host, clock: clock}
	err = closedForwardingServeDirectAdmissionUntil(t, server, serverKey, receiver, closedRestrictionToken(t, fixture), leaseDeadline)
	if !errors.Is(err, host.release) || !strings.Contains(err.Error(), "lease") {
		t.Fatalf("expired refusal lost cleanup or primary: %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("expired ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	host.release = nil
	leaseDeadline = fixture.now.Add(2 * time.Second)
	calls.Store(0)
	freshSpends, openErr := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() { _ = freshSpends.Close() })
	freshLimits, openErr := route.NewClosedDutyLimits(fixture.config.now)
	if openErr != nil {
		t.Fatal(openErr)
	}
	server = &closedForwardingServer{config: fixture.config, certificate: certificate, spends: freshSpends, limits: freshLimits, host: host, clock: clock}
	err = closedForwardingServeDirectAdmissionUntil(t, server, serverKey, receiver, closedRestrictionToken(t, fixture), leaseDeadline)
	if err == nil || errors.Is(err, cleanup) || !strings.Contains(err.Error(), "lease") {
		t.Fatalf("healthy expired refusal = %v", err)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("healthy expired ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
}

func TestClosedForwardingServeDirectRetainsSpendStorageCleanupFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := rendezvousCertificate(t, 243, "forwarding-storage-cleanup-server")
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
	root := t.TempDir()
	spends, err := route.OpenClosedSpendLedger(root, route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, spends: spends, limits: limits, host: host, clock: fixture.config.now}
	journal := filepath.Join(root, "closed-token-spends")
	saved := journal + ".saved"
	if err = os.Rename(journal, saved); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(journal, 0o700); err != nil {
		t.Fatal(err)
	}
	token := closedRestrictionToken(t, fixture)
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token)
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "token") {
		t.Fatalf("storage refusal lost cleanup or primary: %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("storage refusal ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	host.release = nil
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, closedRestrictionToken(t, fixture))
	if err == nil || errors.Is(err, cleanup) || !strings.Contains(err.Error(), "token") {
		t.Fatalf("healthy storage refusal = %v", err)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("healthy storage ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	if err = os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(saved, journal); err != nil {
		t.Fatal(err)
	}
	if err = spends.Close(); !errors.Is(err, syscall.EISDIR) {
		t.Fatalf("failed spend owner close = %v, want EISDIR", err)
	}
}

func TestClosedForwardingServeDirectRetainsCapacityCleanupFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := rendezvousCertificate(t, 244, "forwarding-capacity-cleanup-server")
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
	var verificationTicks atomic.Int32
	limits, err := route.NewClosedDutyLimits(func() time.Time { return fixture.now.Add(time.Duration(verificationTicks.Add(1)/100) * time.Second) })
	if err != nil {
		t.Fatal(err)
	}
	held := fillClosedForwardingDutyCapacity(t, receiver, limits, fixture.now)
	t.Cleanup(func() {
		for _, lease := range held {
			_ = lease.Release()
		}
	})
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, spends: spends, limits: limits, host: host, clock: fixture.config.now}
	token := closedRestrictionToken(t, fixture)
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token)
	if !errors.Is(err, cleanup) || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity refusal lost cleanup or primary: %v", err)
	}
	if host.reserved.Load() != 1 || host.released.Load() != 1 {
		t.Fatalf("capacity refusal ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
	host.release = nil
	err = closedForwardingServeDirectAdmission(t, server, serverKey, receiver, token)
	if err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("healthy capacity refusal = %v", err)
	}
	if host.reserved.Load() != 2 || host.released.Load() != 2 {
		t.Fatalf("healthy capacity ownership = reserved %d released %d", host.reserved.Load(), host.released.Load())
	}
}

func fillClosedForwardingDutyCapacity(t *testing.T, receiver route.ClosedRoleReceiver, limits *route.ClosedDutyLimits, now time.Time) []route.ClosedAdmission {
	t.Helper()
	spends, err := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{99}, Deadline: receiver.NotAfter}
	body, err := route.EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	held := make([]route.ClosedAdmission, 0, 1024)
	for index := 0; index < cap(held); index++ {
		channel, openErr := route.NewClosedAdmissionChannel(receiver, spends, limits, func(string, []byte, int) ([]byte, error) { return bytes.Repeat([]byte{9}, 32), nil }, func(route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
			return route.ClosedAdmissionApproval{Window: now.Truncate(time.Hour)}, nil
		}, func() time.Time { return now })
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, openErr = channel.Accept(route.ClosedLaneFrame{Kind: 1, Body: body}); openErr != nil {
			t.Fatal(openErr)
		}
		token := make([]byte, 354)
		token[0], token[1] = byte(index>>8), byte(index)
		lease, admitErr := channel.Accept(route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)})
		if admitErr != nil {
			t.Fatalf("capacity filler admission %d = %v", index, admitErr)
		}
		held = append(held, lease)
	}
	return held
}

func closedForwardingServeDirectAdmission(t *testing.T, server *closedForwardingServer, key [32]byte, receiver route.ClosedRoleReceiver, token []byte) error {
	return closedForwardingServeDirectAdmissionUntil(t, server, key, receiver, token, receiver.NotAfter)
}

func closedForwardingServeDirectAccepted(t *testing.T, server *closedForwardingServer, key [32]byte, receiver route.ClosedRoleReceiver, token []byte) (net.Conn, <-chan error) {
	t.Helper()
	serverRaw, clientRaw := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	result := make(chan error, 1)
	go func() {
		secured, err := route.AcceptClosedRoleTLS(t.Context(), serverRaw, server.certificate, deadline)
		if err == nil {
			err = server.serveDirect(t.Context(), secured, nil, [32]byte{}, route.ClosedChildOrdinary, nil)
		}
		result <- err
	}()
	joined := false
	defer func() {
		if joined {
			return
		}
		_ = clientRaw.Close()
		_ = serverRaw.Close()
		<-result
	}()
	client, err := route.OpenClosedRoleTLS(t.Context(), clientRaw, key, deadline)
	if err != nil {
		t.Fatalf("inner client TLS = %v", err)
	}
	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{18}, Deadline: receiver.NotAfter}
	body, err := route.EncodeClosedHello(hello)
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 1, Body: body})
	}
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)})
	}
	if err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	frame, err := route.ReadClosedLaneFrame(client)
	if err != nil || frame.Kind != 5 {
		_ = client.Close()
		t.Fatalf("forwarding acceptance = %+v / %v", frame, err)
	}
	joined = true
	return client, result
}

func closedForwardingServeDirectAdmissionUntil(t *testing.T, server *closedForwardingServer, key [32]byte, receiver route.ClosedRoleReceiver, token []byte, admissionDeadline time.Time) error {
	t.Helper()
	serverRaw, clientRaw := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	result := make(chan error, 1)
	go func() {
		secured, err := route.AcceptClosedRoleTLS(t.Context(), serverRaw, server.certificate, deadline)
		if err == nil {
			err = server.serveDirect(t.Context(), secured, nil, [32]byte{}, route.ClosedChildOrdinary, nil)
		}
		result <- err
	}()
	client, err := route.OpenClosedRoleTLS(t.Context(), clientRaw, key, deadline)
	if err != nil {
		_ = clientRaw.Close()
		_ = serverRaw.Close()
		serverErr := <-result
		return errors.Join(err, serverErr)
	}
	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{17}, Deadline: admissionDeadline}
	body, err := route.EncodeClosedHello(hello)
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 1, Body: body})
	}
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)})
	}
	var frame route.ClosedLaneFrame
	if err == nil {
		frame, err = route.ReadClosedLaneFrame(client)
	}
	_ = client.Close()
	serverErr := <-result
	if err == nil {
		if frame.Kind != 5 {
			return errors.New("closed forwarding admission was not accepted")
		}
		if serverErr != nil && !closedForwardingEOFOnly(serverErr) {
			return serverErr
		}
		return nil
	}
	if serverErr != nil {
		return serverErr
	}
	return err
}

func closedForwardingEOFOnly(err error) bool {
	if err == nil || err == io.EOF {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		unwrapped := joined.Unwrap()
		return len(unwrapped) != 0 && allClosedForwardingEOF(unwrapped)
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return closedForwardingEOFOnly(unwrapped)
	}
	return false
}

func allClosedForwardingEOF(errors []error) bool {
	for _, err := range errors {
		if !closedForwardingEOFOnly(err) {
			return false
		}
	}
	return true
}
