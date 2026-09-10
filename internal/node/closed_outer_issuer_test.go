//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestClosedIssuerServesBootstrapInsideStateAuthorizedNodeCarrier(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Truncate(time.Hour).Add(time.Hour)
	serverCertificate, serverKey := rendezvousCertificate(t, 241, "closed-outer-issuer")
	clientCertificate, clientKey := rendezvousCertificate(t, 242, "closed-outer-peer")
	network, issuerID, peerID := [32]byte{61}, [32]byte{62}, [32]byte{63}
	generation, digest := sha256.Sum256([]byte("outer issuer generation")), sha256.Sum256([]byte("outer issuer digest"))
	root := filepath.Join(t.TempDir(), "issuer")
	receipt, err := credential.InitializeClosedIssuerRoot(credential.ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: serverCertificate.PrivateKey.(ed25519.PrivateKey), NotBefore: now.Truncate(time.Hour), NotAfter: until, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := credential.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(serverKey[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: generation, StateDigest: digest, Digest: sha256.Sum256([]byte("outer profile")),
		IssuanceAuthorityKey: [32]byte{64}, IssuerNodeID: issuerID, IssuerDutyGeneration: 8, Epoch: 4, NotBefore: now.Truncate(time.Hour), NotAfter: until, TokenKeyCount: uint8(len(issuerProfile.Keys))}
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	endpoint := reserveAddress(t)
	recordDigest := [32]byte{65}
	snapshot := dutyFacts{Generation: hex.EncodeToString(generation[:]), NetworkID: network, Epoch: profile.Epoch, Digest: digest, EpochValidFrom: profile.NotBefore,
		ValidUntil: until, Profile: route.ClosedRouteProfile, Fresh: true, RecordPresent: true, NodeID: issuerID, NodePublicKey: serverKey, RecordGeneration: profile.IssuerDutyGeneration,
		RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until, DeclaredFamily: "closed-issuer-family", ProbeEndpoint: endpoint, CarrierProfile: string(route.ClosedCarrierTCP), Assignment: "rendezvous",
		CandidateCount: 1, Candidates: [64]dutyCandidate{{NodeID: peerID, PublicKey: clientKey, RecordDigest: [32]byte{66}, Endpoint: "127.0.0.1:41001", CarrierProfile: string(route.ClosedCarrierTCP), ValidUntil: until, AssignmentNotAfter: until}}}
	events := make(chan Event, 16)
	config := Config{NetworkID: network, NodeID: issuerID, IdentityKey: serverCertificate.PrivateKey.(ed25519.PrivateKey), Current: func() (DutyView, error) { return snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return profile, true }, CurrentClosedRoute: func() (state.ClosedRouteView, bool) {
			view := state.ClosedRouteView{Profile: profile, NodeCount: 2}
			view.Nodes[0] = state.ClosedRouteNodeView{NodeID: issuerID, RecordDigest: recordDigest, RoleDomain: 2, Subrole: 6, DutyGeneration: profile.IssuerDutyGeneration}
			view.Nodes[1] = state.ClosedRouteNodeView{NodeID: peerID, RecordDigest: snapshot.Candidates[0].RecordDigest, RoleDomain: 1, Subrole: 1, DutyGeneration: 9}
			return view, true
		}, ClosedIssuer: ClosedIssuerProfile{Root: root, AdmissionRoot: t.TempDir(), Certificate: serverCertificate, ConnectionLimit: 2, DrainTimeout: time.Second},
		PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil }, Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	resolved, err := resolveConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if !closedSharedPeerCurrent(resolved, snapshot, clientKey, time.Now()) {
		t.Fatal("fixture does not authorize outer Node certificate")
	}
	if _, available := closedRouteReceiver(resolved, snapshot, route.ClosedPurposeIssuer, time.Now()); !available {
		t.Fatal("fixture does not authorize local issuer receiver")
	}
	runContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { _, runErr := Run(runContext, config); runDone <- runErr }()
	waitForStateEvent(t, events, "READY")
	deadline := time.Now().Add(10 * time.Second)
	outer, err := route.OpenClosedNodeCarrier(t.Context(), route.ClosedNodeCarrierRequest{CarrierProfile: route.ClosedCarrierTCP, Endpoint: endpoint, Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	outerHello := route.ClosedHello{NetworkID: network, StateGeneration: generation, StateDigest: digest, ProfileDigest: profile.Digest, RecipientNodeID: issuerID, RecipientDutyGeneration: profile.IssuerDutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{67}, Deadline: now.Add(9 * time.Second)}
	body, err := route.EncodeClosedHello(outerHello)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if accepted, err := route.ReadClosedLaneFrame(outer); err != nil || accepted.Kind != 5 {
		t.Fatalf("outer accept = %+v / %v", accepted, err)
	}
	open, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: issuerID, NextDutyGeneration: profile.IssuerDutyGeneration, Purpose: route.ClosedPurposeIssuer, Deadline: now.Add(8 * time.Second)}, route.ClosedChildIssuerBootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: open}); err != nil {
		t.Fatal(err)
	}
	innerRaw := &outerTestInnerConn{outer: outer, lane: 1}
	inner, err := route.OpenClosedRoleTLS(t.Context(), innerRaw, serverKey, deadline)
	if err != nil {
		t.Fatal(err)
	}
	defer inner.Close()
	innerHello := outerHello
	innerHello.Purpose, innerHello.ChannelNonce = route.ClosedPurposeIssuer, [32]byte{68}
	innerHello.Deadline = now.Add(8 * time.Second)
	body, err = route.EncodeClosedHello(innerHello)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	accepted, err := route.ReadClosedLaneFrame(inner)
	if err != nil || accepted.Kind != 5 {
		t.Fatalf("inner issuer accept = %+v / %v", accepted, err)
	}
	_ = outer.Close()
	cancel()
	select {
	case <-runDone:
	case <-time.After(testLifecycleWait):
		t.Fatal("issuer did not drain after outer client close")
	}
}

type outerTestInnerConn struct {
	outer   route.Carrier
	lane    uint32
	mu      sync.Mutex
	inbound []byte
}

func (connection *outerTestInnerConn) Read(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for len(connection.inbound) == 0 {
		frame, err := route.ReadClosedLaneFrame(connection.outer)
		if err != nil {
			return 0, err
		}
		if frame.Kind == 6 && frame.Lane == connection.lane {
			connection.inbound = append(connection.inbound, frame.Body...)
			continue
		}
		if frame.Kind == 9 && frame.Lane == connection.lane {
			return 0, errors.New("inner lane closed")
		}
	}
	count := copy(value, connection.inbound)
	connection.inbound = connection.inbound[count:]
	return count, nil
}
func (connection *outerTestInnerConn) Write(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for rest := value; len(rest) != 0; {
		count := len(rest)
		if count > 16<<10 {
			count = 16 << 10
		}
		if err := route.WriteClosedLaneFrame(connection.outer, route.ClosedLaneFrame{Kind: 6, Lane: connection.lane, Body: rest[:count]}); err != nil {
			return 0, err
		}
		rest = rest[count:]
	}
	return len(value), nil
}
func (connection *outerTestInnerConn) Close() error                     { return nil }
func (connection *outerTestInnerConn) LocalAddr() net.Addr              { return outerTestAddr{} }
func (connection *outerTestInnerConn) RemoteAddr() net.Addr             { return outerTestAddr{} }
func (connection *outerTestInnerConn) SetDeadline(time.Time) error      { return nil }
func (connection *outerTestInnerConn) SetReadDeadline(time.Time) error  { return nil }
func (connection *outerTestInnerConn) SetWriteDeadline(time.Time) error { return nil }

type outerTestAddr struct{}

func (outerTestAddr) Network() string { return "test" }
func (outerTestAddr) String() string  { return "outer-test" }
