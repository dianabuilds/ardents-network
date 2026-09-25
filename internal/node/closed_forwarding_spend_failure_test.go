//go:build linux

package node

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedForwardingStartRefusesAmbiguousSpendJournal(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, serverKey := nodeCertificate(t, 253, "forwarding-recovery-server")
	fixture.snapshot.ProbeEndpoint = reserveAddress(t)
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	root := filepath.Join(t.TempDir(), "spends")
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: root, Certificate: certificate, ConnectionLimit: 2, DrainTimeout: time.Second,
		AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, HostingRoot: closedForwardingHostingRoot(t)}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	spends, err := route.OpenClosedSpendLedger(root, route.ClosedSpendBinding{NetworkID: fixture.receiver.NetworkID, ProfileDigest: fixture.receiver.ProfileDigest,
		ReceiverNodeID: fixture.receiver.NodeID, ReceiverDutyGeneration: fixture.receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	if err := spends.Close(); err != nil {
		t.Fatal(err)
	}
	clean, err := startClosedForwarding(fixture.config, fixture.snapshot)
	if err != nil {
		t.Fatalf("clean receiver startup = %v", err)
	}
	clean.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := clean.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, "closed-token-spends")
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	tail := append(make([]byte, 41), closedForwardingCommittedSpendRecord(fixture.now)...)
	if err := os.WriteFile(journal, append(before, tail...), 0o600); err != nil {
		t.Fatal(err)
	}
	if server, err := startClosedForwarding(fixture.config, fixture.snapshot); err == nil {
		server.Stop()
		t.Fatal("receiver started with ambiguous spend journal")
	}
	after, err := os.ReadFile(journal)
	if err != nil || string(after) != string(append(before, tail...)) {
		t.Fatalf("startup refusal changed journal: %v", err)
	}
}

func closedForwardingCommittedSpendRecord(window time.Time) []byte {
	record := make([]byte, 41)
	record[0] = 1
	binary.BigEndian.PutUint64(record[32:40], uint64(window.Truncate(time.Hour).Unix()))
	record[40] = 1
	return record
}

func TestClosedForwardingServerRefusesAfterJournalMutationFailure(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	serverCertificate, serverKey := nodeCertificate(t, 251, "forwarding-spend-server")
	peerCertificate, peerKey := nodeCertificate(t, 252, "forwarding-spend-peer")
	fixture.snapshot.ProbeEndpoint = reserveAddress(t)
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.snapshot.Candidates[0].PublicKey = peerKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	root := filepath.Join(t.TempDir(), "spends")
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: root, Certificate: serverCertificate, ConnectionLimit: 2, DrainTimeout: time.Second,
		AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, HostingRoot: closedForwardingHostingRoot(t)}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	token := closedRestrictionToken(t, fixture)
	server, err := startClosedForwarding(fixture.config, fixture.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Drain(ctx); errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("forwarding server did not drain: %v", err)
		}
	})

	journal := filepath.Join(root, "closed-token-spends")
	saved := journal + ".saved"
	if err := os.Rename(journal, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(journal, 0o700); err != nil {
		t.Fatal(err)
	}
	first := openClosedForwardingAdmission(t, fixture, peerCertificate, serverKey, token, 1)
	if err := first.admit(); !errors.Is(err, io.EOF) {
		t.Fatal("journal mutation failure admitted forwarding")
	} else {
		t.Logf("first admission refusal: %v", err)
	}
	_ = first.close()
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, journal); err != nil {
		t.Fatal(err)
	}

	second := openClosedForwardingAdmission(t, fixture, peerCertificate, serverKey, token, 2)
	defer second.close()
	if err := second.admit(); !errors.Is(err, io.EOF) {
		t.Fatal("restored journal admitted after the owner terminalized")
	} else {
		t.Logf("second admission refusal: %v", err)
	}
}

type closedForwardingAdmissionClient struct {
	outer route.Carrier
	inner route.Carrier
	hello ardp.Hello
	token []byte
}

func openClosedForwardingAdmission(t *testing.T, fixture *closedBootstrapFixture, certificate tls.Certificate, serverKey [32]byte, token []byte, nonce byte) *closedForwardingAdmissionClient {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	outer, err := route.OpenClosedNodeCarrier(t.Context(), route.ClosedNodeCarrierRequest{CarrierProfile: route.ClosedCarrierTCP, Endpoint: fixture.snapshot.ProbeEndpoint, Certificate: certificate, ExpectedPeerKey: serverKey, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = outer.Close() })
	hello := ardp.Hello{NetworkID: fixture.receiver.NetworkID, StateGeneration: fixture.receiver.StateGeneration, StateDigest: fixture.receiver.StateDigest, ProfileDigest: fixture.receiver.ProfileDigest, RecipientNodeID: fixture.receiver.NodeID, RecipientDutyGeneration: fixture.receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{nonce}, Deadline: fixture.now.Add(8 * time.Second)}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if frame, err := ardp.ReadFrame(outer); err != nil || frame.Kind != 5 {
		t.Fatalf("outer accept = %+v / %v", frame, err)
	}
	open, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: fixture.receiver.NodeID, NextDutyGeneration: fixture.receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: hello.Deadline}, route.ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 4, Lane: 1, Body: open}); err != nil {
		t.Fatal(err)
	}
	inner, err := route.OpenClosedRoleTLS(t.Context(), &outerTestInnerConn{outer: outer, lane: 1}, serverKey, deadline)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inner.Close() })
	body, err = ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(inner, ardp.Frame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	return &closedForwardingAdmissionClient{outer: outer, inner: inner, hello: hello, token: token}
}

func (client *closedForwardingAdmissionClient) admit() error {
	if err := ardp.WriteFrame(client.inner, ardp.Frame{Kind: 2, Body: append([]byte{2}, client.token...)}); err != nil {
		return err
	}
	frame, err := ardp.ReadFrame(client.inner)
	if err != nil {
		return err
	}
	if frame.Kind != 5 {
		return errors.New("forwarding admission was unavailable")
	}
	return nil
}
func (client *closedForwardingAdmissionClient) close() error {
	return errors.Join(client.inner.Close(), client.outer.Close())
}
