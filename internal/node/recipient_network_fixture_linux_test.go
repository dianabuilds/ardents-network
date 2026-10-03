//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"net"
	"path/filepath"
	"testing"
	"time"

	admissiongrammar "github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func privateRecipientFixtureStart(t *testing.T) (time.Time, time.Time) {
	t.Helper()
	for {
		now := time.Now().UTC()
		start, end := privateRecipientFixtureWindow(now)
		if !start.After(now) {
			return now.Truncate(time.Second), end
		}
		// Wait for a valid test issuance window; do not change product clocks,
		// redeem a future token early, or drop the test as an unavailable skip.
		timer := time.NewTimer(time.Until(start))
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal("recipient fixture window canceled")
		case <-timer.C:
		}
	}
}

// State, offline Permission authority and existing Publication acknowledgement
// are fixtures. The recipient Run lifecycle, Carrier, nested TLS, issuer crypto,
// admission spend and Descriptor Store are real. This does not qualify private
// Introduction registration or Endpoint Publisher readiness.
type resolutionNetworkFixture struct {
	observe       func(net.Conn) net.Conn
	admissionRoot string
	profile       state.ClosedProfileView
	carrier       routecarrier.CarrierProfile
	endpoint      string
	certificate   tls.Certificate
	receiver      route.ClosedRoleReceiver
	serverKey     [32]byte
	tokens        [][]byte
	supplementary map[uint8][][]byte
	current       publication.Current
	signer        ed25519.PrivateKey
	introduction  reachability.PrivateIntroduction
	root          string
	restart       func()
}

func newResolutionNetworkFixture(t *testing.T, carrier routecarrier.CarrierProfile) *resolutionNetworkFixture {
	t.Helper()
	return newPrivateRecipientNetworkFixture(t, carrier, ardp.PurposeReachability, 1)
}

func newPrivateRecipientNetworkFixture(t *testing.T, carrier routecarrier.CarrierProfile, purpose ardp.Purpose, class uint8, extraClasses ...uint8) *resolutionNetworkFixture {
	t.Helper()
	return newPrivateRecipientNetworkFixtureWithStart(t, carrier, purpose, class, nil, extraClasses...)
}

func newPrivateRecipientNetworkFixtureWithStart(t *testing.T, carrier routecarrier.CarrierProfile, purpose ardp.Purpose, class uint8, startReceiver func(Config) (func() error, error), extraClasses ...uint8) *resolutionNetworkFixture {
	t.Helper()
	now, until := privateRecipientFixtureStart(t)
	serverCert, serverKey := nodeCertificate(t, 251, "resolution")
	clientCert, clientKey := nodeCertificate(t, 252, "resolution-peer")
	authorityPublic, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	network, nodeID, peerID, introID, issuerID := [32]byte{71}, [32]byte{72}, [32]byte{73}, [32]byte{74}, [32]byte{75}
	root := filepath.Join(t.TempDir(), "issuer")
	receipt, err := admissionissuer.InitializeClosedIssuerRoot(admissionissuer.ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerID,
		IdentityKey: serverCert.PrivateKey.(ed25519.PrivateKey), NotBefore: now.Truncate(time.Hour), NotAfter: until, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := admissiongrammar.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(serverKey[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: network, StateGeneration: sha256.Sum256([]byte("resolution generation")), StateDigest: sha256.Sum256([]byte("resolution State")),
		Digest: sha256.Sum256([]byte("resolution profile")), IssuerNodeID: issuerID, IssuerDutyGeneration: 8, Epoch: 4,
		NotBefore: now.Truncate(time.Hour), NotAfter: until, TokenKeyCount: uint8(len(inventory.Keys))}
	copy(profile.IssuanceAuthorityKey[:], authorityPublic)
	for index, key := range inventory.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	endpoint := reserveClosedBootstrapAddress(t, carrier)
	snapshot := state.NodeDuty{Generation: hex.EncodeToString(profile.StateGeneration[:]), NetworkID: network, Epoch: profile.Epoch, Digest: profile.StateDigest,
		EpochValidFrom: profile.NotBefore, ValidUntil: until, Profile: routecarrier.ClosedRouteProfile, Fresh: true, RecordPresent: true,
		NodeID: nodeID, NodePublicKey: serverKey, RecordGeneration: 9, RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until,
		DeclaredFamily: "resolution-family", ProbeEndpoint: endpoint, CarrierProfile: string(carrier), Assignment: "rendezvous", CandidateCount: 2}
	snapshot.Candidates[0] = state.NodeDutyCandidate{NodeID: peerID, PublicKey: clientKey, RecordDigest: [32]byte{76}, Endpoint: "127.0.0.1:41001", CarrierProfile: string(carrier), ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until}
	snapshot.Candidates[1] = state.NodeDutyCandidate{NodeID: introID, PublicKey: [32]byte{77}, RecordDigest: [32]byte{78}, Endpoint: "127.0.0.1:41002", CarrierProfile: string(carrier), ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until}
	view := state.ClosedRouteView{Profile: profile, NodeCount: 3}
	view.Nodes[0] = state.ClosedRouteNodeView{NodeID: nodeID, RecordDigest: [32]byte{79}, RoleDomain: 2, Subrole: 5, DutyGeneration: 9}
	view.Nodes[1] = state.ClosedRouteNodeView{NodeID: peerID, RecordDigest: snapshot.Candidates[0].RecordDigest, RoleDomain: 1, Subrole: 1, DutyGeneration: 10}
	view.Nodes[2] = state.ClosedRouteNodeView{NodeID: introID, RecordDigest: snapshot.Candidates[1].RecordDigest, RoleDomain: 4, Subrole: 3, DutyGeneration: 11}
	if purpose == ardp.PurposeIntroduction {
		snapshot.Assignment = "introduction"
		view.Nodes[0].RoleDomain, view.Nodes[0].Subrole = 4, 3
	}
	if purpose == ardp.PurposeDataJoin {
		view.Nodes[0].RoleDomain, view.Nodes[0].Subrole = 2, 4
	}
	events := make(chan Event, 32)
	config := Config{HostingRoot: hostingFixtureRoot(t), NetworkID: network, NodeID: nodeID, IdentityKey: serverCert.PrivateKey.(ed25519.PrivateKey), Current: func() (state.NodeDuty, error) { return snapshot, nil },
		CurrentClosedProfile: func() (state.ClosedProfileView, bool) { return profile, true }, CurrentClosedRoute: func() (state.ClosedRouteView, error) { return view, nil },
		ClosedResolution: ClosedResolutionProfile{Root: t.TempDir(), AdmissionRoot: t.TempDir(), Certificate: serverCert, ConnectionLimit: 2, DrainTimeout: time.Second},
		PollInterval:     10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	if purpose == ardp.PurposeIntroduction {
		config.ClosedResolution = ClosedResolutionProfile{}
		config.ClosedIntroduction = ClosedIntroductionProfile{AdmissionRoot: t.TempDir(), Certificate: serverCert, ConnectionLimit: 8, DrainTimeout: time.Second}
	}
	if purpose == ardp.PurposeDataJoin {
		config.ClosedResolution = ClosedResolutionProfile{}
		config.ClosedDataJoin = ClosedDataJoinProfile{HostingRoot: config.HostingRoot, AdmissionRoot: t.TempDir(), Certificate: serverCert, ConnectionLimit: 8, DrainTimeout: time.Second}
	}
	resolved, err := resolveConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	receiver, ok := nodeAuthority(resolved).Receiver(snapshot, purpose, time.Now())
	if !ok || !nodeAuthority(resolved).PeerCurrent(snapshot, clientKey, time.Now()) {
		t.Fatal("invalid resolution State fixture")
	}
	fixture := &resolutionNetworkFixture{profile: profile, carrier: carrier, endpoint: endpoint, certificate: clientCert, receiver: receiver, serverKey: serverKey, root: config.ClosedResolution.Root}
	fixture.admissionRoot = config.ClosedIntroduction.AdmissionRoot
	fixture.tokens = privateRecipientTokens(t, root, profile, authority, receiver, class)
	fixture.supplementary = make(map[uint8][][]byte)
	for _, extra := range extraClasses {
		fixture.supplementary[extra] = privateRecipientTokens(t, root, profile, authority, receiver, extra)
	}
	if purpose == ardp.PurposeReachability {
		fixture.current, fixture.signer = resolutionPublication(t, network, now, until)
		fixture.introduction = reachability.PrivateIntroduction{Revision: 1, NodeID: introID, Slot: [32]byte{81}, RecipientKey: [32]byte{82}, NotBefore: now, NotAfter: minResolutionTime(now.Add(30*time.Second), until)}
	}
	var cancel context.CancelFunc
	var done chan error
	var stopReceiver func() error
	start := func() {
		if startReceiver != nil {
			var err error
			stopReceiver, err = startReceiver(config)
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		done = make(chan error, 1)
		go func() { _, runErr := Run(ctx, config); done <- runErr }()
		waitForStateEvent(t, events, "READY")
	}
	stop := func() bool {
		if stopReceiver != nil {
			if err := stopReceiver(); err != nil {
				t.Error(err)
				return false
			}
			return true
		}
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("recipient shutdown: %v", err)
				return false
			}
			return true
		case <-time.After(testLifecycleWait):
			t.Error("recipient did not join")
			return false
		}
	}
	start()
	fixture.restart = func() {
		t.Helper()
		if !stop() {
			t.Fatal("cannot restart an unjoined recipient")
		}
		start()
	}
	t.Cleanup(func() {
		if !stop() {
			return
		}
		if purpose == ardp.PurposeIntroduction {
			ledger, err := spending.Open(config.ClosedIntroduction.AdmissionRoot, spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
			if err != nil {
				t.Error(err)
			} else if err := ledger.Close(); err != nil {
				t.Error(err)
			}
			return
		}
		if purpose == ardp.PurposeDataJoin {
			ledger, err := spending.Open(config.ClosedDataJoin.AdmissionRoot, spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
			if err != nil {
				t.Error(err)
			} else if err := ledger.Close(); err != nil {
				t.Error(err)
			}
			return
		}
		reopened, err := reachability.OpenStore(reachability.StoreConfig{Root: fixture.root, NetworkID: network})
		if err != nil {
			t.Errorf("resolution retained root after shutdown: %v", err)
			return
		}
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	return fixture
}

func privateRecipientTokens(t *testing.T, root string, profile state.ClosedProfileView, authority ed25519.PrivateKey, receiver route.ClosedRoleReceiver, class uint8) [][]byte {
	t.Helper()
	issuer, err := admissionissuer.OpenClosedTokenIssuer(admissionissuer.ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID, CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	}()
	public, holder, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holder)
	permission := admissiongrammar.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: [32]byte{83, class}, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Maxima: [3]uint32{}, Signature: [64]byte{1}}
	permission.Maxima[class-1] = 12
	copy(permission.HolderKey[:], public)
	raw, err := admissiongrammar.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	copy(permission.Signature[:], ed25519.Sign(authority, append([]byte("ardents-issuance-permission-v1\x00"), raw[:len(raw)-64]...)))
	contexts := make([]admissiontoken.ClosedTokenContext, 12)
	for i := range contexts {
		contexts[i] = admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
			ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: class, WindowStart: profile.NotBefore}
	}
	pending, err := admissiontoken.PrepareClosedTokenBatch(admissiontoken.ClosedTokenBatchConfig{Profile: profile, Contexts: contexts, Permission: permission, HolderKey: holder, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Discard()
	nonce := [32]byte{84}
	request, err := terminal.EncodeIssuanceRequest(nonce, pending.Request())
	if err != nil {
		t.Fatal(err)
	}
	result, err := issuer.IssueTerminalOperation(request)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pending.FinalizeTerminalOperation(nonce, result)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, token := range tokens {
			clear(token)
		}
	})
	return tokens
}

func resolutionPublication(t *testing.T, network [32]byte, now, until time.Time) (publication.Current, ed25519.PrivateKey) {
	t.Helper()
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	instancePublic, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(signer) })
	var instance [32]byte
	copy(instance[:], instancePublic)
	grant, err := (publication.Credential{InstancePublic: instance, Generation: 1, NotBefore: now.Add(-time.Second).Unix(), NotAfter: until.Unix(), NetworkID: network, Capabilities: 3}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := publication.Open(publication.Config{Root: t.TempDir(), NetworkID: network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}()
	current, err := owner.Publish(t.Context(), publication.PublishInput{Credential: grant, InstanceSigner: signer, Acknowledgement: []byte("explicit pre-existing Publication fixture"), At: now})
	if err != nil {
		t.Fatal(err)
	}
	return current, signer
}
func minResolutionTime(first, second time.Time) time.Time {
	if first.Before(second) {
		return first
	}
	return second
}

func (fixture *resolutionNetworkFixture) openTerminal(ctx context.Context, token []byte, class uint8) (net.Conn, func(), error) {
	end := minResolutionTime(time.Now().UTC().Add(90*time.Second).Truncate(time.Second), fixture.profile.NotAfter)
	outer, err := routecarrier.OpenClosedNodeCarrier(ctx, routecarrier.ClosedNodeCarrierRequest{CarrierProfile: fixture.carrier, Endpoint: fixture.endpoint, Certificate: fixture.certificate, ExpectedPeerKey: fixture.serverKey, Deadline: end})
	if err != nil {
		return nil, nil, fmt.Errorf("outer dial: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = outer.Close()
		}
	}()
	hello := ardp.Hello{NetworkID: fixture.profile.NetworkID, StateGeneration: fixture.profile.StateGeneration, StateDigest: fixture.profile.StateDigest,
		ProfileDigest: fixture.profile.Digest, RecipientNodeID: fixture.receiver.NodeID, RecipientDutyGeneration: fixture.receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: end}
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, nil, err
	}
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		return nil, nil, err
	}
	if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 1, Body: body}); err != nil {
		return nil, nil, err
	}
	accepted, err := ardp.ReadFrame(outer)
	if err != nil || accepted.Kind != 5 {
		return nil, nil, fmt.Errorf("outer accept: kind=%d error=%v", accepted.Kind, err)
	}
	open, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: fixture.receiver.NodeID, NextDutyGeneration: fixture.receiver.DutyGeneration, Purpose: fixture.receiver.ExpectedPurpose, Deadline: end}, route.ClosedChildOrdinary)
	if err != nil {
		return nil, nil, err
	}
	if err := ardp.WriteFrame(outer, ardp.Frame{Kind: 4, Lane: 1, Body: open}); err != nil {
		return nil, nil, err
	}
	secured, err := routecarrier.OpenClosedRoleTLS(ctx, &outerTestInnerConn{outer: outer, lane: 1}, fixture.serverKey, end)
	if err != nil {
		return nil, nil, fmt.Errorf("inner TLS: %w", err)
	}
	var inner net.Conn = secured
	if fixture.observe != nil {
		inner = fixture.observe(inner)
	}
	hello.Purpose = fixture.receiver.ExpectedPurpose
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, nil, err
	}
	body, err = ardp.EncodeHello(hello)
	if err != nil {
		return nil, nil, err
	}
	if err := ardp.WriteFrame(inner, ardp.Frame{Kind: 1, Body: body}); err != nil {
		return nil, nil, err
	}
	admit := append([]byte{class}, token...)
	defer clear(admit)
	if err := ardp.WriteFrame(inner, ardp.Frame{Kind: 2, Body: admit}); err != nil {
		return nil, nil, err
	}
	accepted, err = ardp.ReadFrame(inner)
	if err != nil {
		return nil, nil, fmt.Errorf("Control admission: %w", err)
	}
	if status, credit, err := ardp.DecodeAcceptFrame(accepted); err != nil || status != 0 || credit != 64<<10 {
		return nil, nil, fmt.Errorf("Control accept: status=%d credit=%d error=%v", status, credit, err)
	}

	success = true
	return inner, func() { _ = outer.Close() }, nil
}
