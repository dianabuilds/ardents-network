package endpoint

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// TestUserRouteCredentialCompletionUsesEndpointJournal proves the whole
// composition seam: Route acquires Endpoint's durable credential adapter,
// burns the grant after an unsuccessful Introduction delivery, and spends it
// only after the exact delivered result.
func TestUserRouteCredentialCompletionUsesEndpointJournal(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome byte
		phase   transitAcquisitionPhase
		attach  bool
	}{
		{name: "unavailable delivery burns", outcome: route.IntroductionUnavailable, phase: transitBurned},
		{name: "exact delivery spends", outcome: route.IntroductionDelivered, phase: transitSpent, attach: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := openUserRouteCredentialFixture(t, test.outcome)
			defer fixture.close()

			attachment, err := fixture.route.Attach(t.Context(), route.Intent{Target: fixture.target})
			if test.attach {
				if err != nil || attachment == nil {
					t.Fatalf("route attachment = %v, %v", attachment, err)
				}
				if err := attachment.Close(); err != nil {
					t.Fatalf("close route attachment: %v", err)
				}
			} else if err == nil || attachment != nil {
				t.Fatalf("unavailable Introduction delivery = %v, %v", attachment, err)
			}
			fixture.wait(t)

			if phase := fixture.endpoint.transitAcquire.introduction.stateForTest().Phase; phase != test.phase {
				t.Fatalf("durable credential phase = %q, want %q", phase, test.phase)
			}
		})
	}
}

type userRouteCredentialFixture struct {
	route             *route.Route
	endpoint          *endpoint
	now               time.Time
	network           [32]byte
	authority         ed25519.PublicKey
	credential        publication.Credential
	instanceSigner    ed25519.PrivateKey
	target            [32]byte
	entry             *userRouteCredentialEntry
	entryCalls        int
	connectionHandler func(net.Conn) error
	serverDone        <-chan error
	close             func()
}

func openUserRouteCredentialFixture(t *testing.T, outcome byte) *userRouteCredentialFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(10 * time.Second)
	network, digest := [32]byte{71}, [32]byte{72}
	const epoch = uint64(73)

	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	introductionHPKEPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var instance, introductionHPKE [32]byte
	copy(instance[:], instancePublic)
	copy(introductionHPKE[:], introductionHPKEPrivate.PublicKey().Bytes())
	publicationCredential, err := (publication.Credential{InstancePublic: instance, IntroductionHPKEPublic: introductionHPKE,
		Generation: 1, NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Minute).Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	publicationOwner, err := publication.Open(publication.Config{Root: publicationStoreRoot(t), NetworkID: network, Authority: authorityPublic,
		Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	current, err := publicationOwner.Publish(t.Context(), publication.PublishInput{Credential: publicationCredential, InstanceSigner: instancePrivate,
		Acknowledgement: []byte("user route credential lifecycle"), At: now})
	if err != nil {
		_ = publicationOwner.Close()
		t.Fatal(err)
	}

	store, err := reachability.OpenStore(reachability.StoreConfig{Root: reachabilityStoreRoot(t), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const gatewayNode = byte(74)
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: [32]byte{gatewayNode}, IdentityKey: gatewayPrivate,
		AssignmentNotAfter: now.Add(time.Minute), Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(reachability.Descriptor, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())

	introductionCertificate, err := route.NewClientCertificate()
	if err != nil {
		t.Fatal(err)
	}
	introductionPublic := testCertificatePublic(t, introductionCertificate)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const introductionNode = byte(75)
	descriptorRaw, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: instancePrivate,
		Introduction: reachability.Introduction{StateDigest: digest, Epoch: epoch, IntroductionNodeID: [32]byte{introductionNode},
			RendezvousNodeID: [32]byte{76}, Reachability: [32]byte{77}, JoinHandle: [32]byte{78}, NotAfter: deadline,
			SubmissionMode: reachability.SubmissionMembershipGrant}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Publish(descriptorRaw, now); err != nil {
		t.Fatal(err)
	}

	issuerCertificate, err := testTransitServerCertificate(now)
	if err != nil {
		t.Fatal(err)
	}
	issuerIdentity, ok := issuerCertificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("issuer certificate has no ed25519 private key")
	}
	issuerClientCertificate, err := route.NewClientCertificate()
	if err != nil {
		t.Fatal(err)
	}
	issuerInitiatorPublic := testCertificatePublic(t, issuerClientCertificate)
	issuerPublic := testCertificatePublic(t, issuerCertificate)
	issuerRoot := credentialIssuerRoot(t)
	const issuerNode = byte(79)
	receipt, err := credential.InitializeIssuerRoot(credential.IssuerRootConfig{Root: issuerRoot, NetworkID: network, NodeID: [32]byte{issuerNode},
		IdentityKey: issuerIdentity, InitiatorNodeID: [32]byte{80}, InitiatorPublicKey: issuerInitiatorPublic,
		AssignmentNotAfter: deadline, Budget: 2, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := credential.DecodeProfile(receipt.Profile)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := credential.OpenIssuerFromRoot(credential.RootIssuerConfig{Root: issuerRoot, NetworkID: network, NodeID: [32]byte{issuerNode},
		IdentityKey: issuerIdentity, Clock: func() time.Time { return now }, CurrentDuty: func() (credential.StateDuty, bool) {
			return credential.StateDuty{Generation: strings.Repeat("a", 64), NetworkID: network, Digest: digest, IssuerNodeID: [32]byte{issuerNode},
				IssuerPublicKey: issuerPublic, InitiatorNodeID: [32]byte{80}, InitiatorPublicKey: issuerInitiatorPublic,
				GrantSignerPublicKey: issuerProfile.GrantSignerPublicKey, ProfileDigest: receipt.ProfileDigest, Epoch: epoch, NotAfter: deadline, Fresh: true}, true
		}})
	if err != nil {
		t.Fatal(err)
	}
	issuerServer := httptest.NewUnstartedServer(issuer.Handler())
	issuerServer.TLS, err = issuer.TLSConfig(issuerCertificate)
	if err != nil {
		t.Fatal(err)
	}
	issuerServer.StartTLS()
	issuerClient := issuerServer.Client()
	base, ok := issuerClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("issuer test client has no HTTP transport")
	}
	issuerTransport := base.Clone()
	issuerTransport.TLSClientConfig = issuerTransport.TLSClientConfig.Clone()
	issuerTransport.TLSClientConfig.InsecureSkipVerify = true // Test-only server identity is fixed by the Issuer fixture.
	issuerTransport.TLSClientConfig.Certificates = []tls.Certificate{issuerClientCertificate}
	issuerClient = &http.Client{Transport: issuerTransport}

	var result *userRouteCredentialFixture
	entryOwner := &userRouteCredentialEntry{contact: entry.Candidate{NodeID: [32]byte{80}, PublicKey: issuerInitiatorPublic,
		FamilyID: sha256.Sum256([]byte("initiator-family")), Endpoint: "127.0.0.1:1"}, errs: make(chan error, 3)}
	entryOwner.handlers = []func(net.Conn) error{
		resolutionRelayHandler(gatewayServer.URL, gatewayServer.Client()),
		credentialRelayHandler(issuerServer.URL, issuerClient),
		func(connection net.Conn) error {
			var handler func(net.Conn) error
			if result != nil {
				handler = result.connectionHandler
			}
			return targetLinkIntroductionRelayHandler(connection, handler)
		},
	}
	var gatewayKey [32]byte
	copy(gatewayKey[:], gatewayPublic)
	view := userRouteCredentialState{epoch: state.ResolutionEpoch{Generation: "user-route-credential-lifecycle", NetworkID: network, Digest: digest, Number: epoch},
		gateway: state.DestinationResolutionGateway{NodeID: [32]byte{gatewayNode}, PublicKey: gatewayKey, Family: [32]byte{82},
			Profile: mustEncodeGatewayProfile(t, gateway.Profile()), AssignmentNotAfter: now.Add(time.Minute)},
		initiator: state.ResolutionCandidate{NodeID: entryOwner.contact.NodeID, PublicKey: entryOwner.contact.PublicKey, Family: "initiator-family",
			Endpoint: entryOwner.contact.Endpoint, Domain: "initiator", AssignmentNotAfter: now.Add(time.Minute)},
		introduction: state.ResolutionCandidate{NodeID: [32]byte{introductionNode}, PublicKey: introductionPublic, Family: "introduction-family",
			Endpoint: listener.Addr().String(), Domain: "introduction", AssignmentNotAfter: now.Add(time.Minute)},
		rendezvous: state.ResolutionCandidate{NodeID: [32]byte{76}, PublicKey: [32]byte{83}, Family: "rendezvous-family",
			Endpoint: "127.0.0.1:2", Domain: "rendezvous", AssignmentNotAfter: now.Add(time.Minute)},
		issuer: state.TransitIssuer{NodeID: [32]byte{issuerNode}, PublicKey: issuerPublic, Family: [32]byte{84}, Profile: receipt.Profile}}
	endpoint, err := newEndpoint(setup{NetworkID: network, BrokerID: [32]byte{85}, ConnectionPrincipal: [32]byte{86},
		TransitAcquisitionRoot: transitAcquisitionRoot(t), CreateTransitAcquisitionRoot: true, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	routeOwner, err := route.Open(route.Config{NetworkID: network, Current: func() (route.StateView, error) { return view, nil }, Entry: entryOwner,
		Credentials: endpoint.acquireUserRouteCredential, Admit: func(context.Context) (func() error, error) { return func() error { return nil }, nil },
		Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	result = &userRouteCredentialFixture{route: routeOwner, endpoint: endpoint, now: now, network: network,
		authority: authorityPublic, credential: publicationCredential, instanceSigner: instancePrivate,
		target: current.Credential.Target, entry: entryOwner, entryCalls: len(entryOwner.handlers), close: func() {
			_ = routeOwner.Close()
			_ = endpoint.Close()
			_ = listener.Close()
			issuerServer.Close()
			_ = issuer.Close()
			gatewayServer.Close()
			_ = store.Close()
			_ = publicationOwner.Close()
		}}
	result.serverDone = serveTargetLinkIntroduction(listener, network, digest, epoch, [32]byte{introductionNode}, deadline,
		introductionCertificate, outcome, nil)
	return result
}

type userRouteCredentialState struct {
	epoch                               state.ResolutionEpoch
	gateway                             state.DestinationResolutionGateway
	initiator, introduction, rendezvous state.ResolutionCandidate
	issuer                              state.TransitIssuer
}

func (view userRouteCredentialState) Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool) {
	return view.epoch, true
}

func (view userRouteCredentialState) Candidate(node [32]byte, _ time.Time, _ time.Time) (state.ResolutionCandidate, bool) {
	for _, candidate := range []state.ResolutionCandidate{view.initiator, view.introduction, view.rendezvous} {
		if candidate.NodeID == node {
			return candidate, true
		}
	}
	return state.ResolutionCandidate{}, false
}

func (view userRouteCredentialState) Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool) {
	return view.gateway, true
}

func (view userRouteCredentialState) CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool) {
	return view.issuer, true
}

type userRouteCredentialEntry struct {
	contact  entry.Candidate
	mu       sync.Mutex
	next     int
	handlers []func(net.Conn) error
	errs     chan error
}

func (owner *userRouteCredentialEntry) Contact() (entry.Candidate, error) { return owner.contact, nil }

func (owner *userRouteCredentialEntry) Acquire(_ context.Context, _ entry.Attempt, _ entry.CandidateOpener) (net.Conn, func() error, error) {
	owner.mu.Lock()
	if owner.next == len(owner.handlers) {
		owner.mu.Unlock()
		return nil, nil, errors.New("unexpected Entry acquisition")
	}
	handler := owner.handlers[owner.next]
	owner.next++
	owner.mu.Unlock()
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		owner.errs <- handler(server)
	}()
	return client, client.Close, nil
}

func (owner *userRouteCredentialEntry) wait(t *testing.T, count int) {
	t.Helper()
	for range count {
		select {
		case err := <-owner.errs:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Entry relay did not finish")
		}
	}
}

func (fixture userRouteCredentialFixture) wait(t *testing.T) {
	t.Helper()
	fixture.entry.wait(t, fixture.entryCalls)
	select {
	case err := <-fixture.serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Introduction server did not finish")
	}
}

func resolutionRelayHandler(gatewayURL string, client *http.Client) func(net.Conn) error {
	return func(connection net.Conn) error {
		setup, err := route.ReadResolutionRelaySetup(connection)
		if err == nil {
			err = route.WriteResolutionRelayReady(connection, route.ResolutionRelayReady{Setup: setup})
		}
		envelope := route.ResolutionRelayEnvelope{}
		if err == nil {
			envelope, err = route.ReadResolutionRelayEnvelope(connection)
		}
		response := reachability.OHTTPResponse{}
		if err == nil {
			response, err = reachability.ForwardOHTTP(context.Background(), gatewayURL, client, envelope.OHTTP)
		}
		if err == nil {
			framing := route.ResolutionOHTTPResponse
			if response.Chunked {
				framing = route.ResolutionOHTTPChunkedResponse
			}
			err = route.WriteResolutionRelayResponse(connection, route.ResolutionRelayResponse{OHTTP: response.Envelope, Framing: framing})
		}
		return err
	}
}

func credentialRelayHandler(issuerURL string, client *http.Client) func(net.Conn) error {
	return func(connection net.Conn) error {
		setup, err := route.ReadCredentialRelaySetup(connection)
		if err == nil {
			err = route.WriteCredentialRelayReady(connection, route.CredentialRelayReady{Setup: setup})
		}
		envelope := route.CredentialRelayEnvelope{}
		if err == nil {
			envelope, err = route.ReadCredentialRelayEnvelope(connection)
		}
		response := []byte(nil)
		if err == nil {
			response, err = credential.ForwardOHTTP(context.Background(), issuerURL, client, envelope.OHTTP)
		}
		if err == nil {
			err = route.WriteCredentialRelayResponse(connection, route.CredentialRelayResponse{OHTTP: response, Framing: route.CredentialOHTTPResponse})
		}
		return err
	}
}

func mustEncodeGatewayProfile(t *testing.T, profile reachability.GatewayProfile) []byte {
	t.Helper()
	raw, err := reachability.EncodeGatewayProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func testCertificatePublic(t *testing.T, certificate tls.Certificate) [32]byte {
	t.Helper()
	public, ok := certificate.PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey)
	if !ok {
		t.Fatal("test certificate has no ed25519 public key")
	}
	var result [32]byte
	copy(result[:], public)
	return result
}

func testTransitServerCertificate(now time.Time) (tls.Certificate, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{SerialNumber: new(big.Int).SetBytes(public[:8]), Subject: pkix.Name{CommonName: "ardents-test-transit"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, nil
}
