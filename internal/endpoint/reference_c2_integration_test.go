package endpoint_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestC2RouteCarriesReferenceSiteBetweenTwoEndpoints(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.CarrierTCP, route.CarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) { testC2RouteCarriesReferenceSite(t, carrier) })
	}
}

func testC2RouteCarriesReferenceSite(t *testing.T, carrier route.CarrierProfile) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	resolutionDeadline := now.Add(5 * time.Second)
	network, digest := c2Identifier(61), c2Identifier(62)
	introductionID, rendezvousID := c2Identifier(63), c2Identifier(64)
	responderID, initiatorID := c2Identifier(65), c2Identifier(66)
	introductionCertificate, introductionPublic := c2Certificate(t, 61, "introduction")
	rendezvousCertificate, rendezvousPublic := c2Certificate(t, 62, "rendezvous")
	responderCertificate, responderPublic := c2Certificate(t, 63, "responder")
	initiatorCertificate, initiatorPublic := c2Certificate(t, 64, "initiator")
	introductionAddress, rendezvousAddress := c2AvailableAddress(t), c2CarrierAddress(t, carrier)
	responderAddress, initiatorAddress := c2AvailableAddress(t), c2AvailableAddress(t)
	join, slotReachability := c2Identifier(67), c2Identifier(68)
	slotAttachment, serviceAttachment, resolutionAttachment := c2Identifier(69), c2Identifier(70), c2Identifier(108)
	slotAuthorization, responderAuthorization := []byte("publisher-slot"), []byte("publisher-responder")
	presentation := entry.Presentation{InviteID: c2Identifier(71), Invite: []byte("entry-invite")}
	gatewayCertificate, gatewayPublic, gatewayPrivate := c2GatewayCertificate(t)
	gatewayID := c2Identifier(109)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: gatewayID, IdentityKey: gatewayPrivate,
		AssignmentNotAfter: deadline, Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(value reachability.Descriptor, at time.Time) bool {
			return at.Equal(now) && value.Introduction.StateDigest == digest && value.Introduction.Epoch == 10 &&
				value.Introduction.IntroductionNodeID == introductionID && value.Introduction.RendezvousNodeID == rendezvousID
		}})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewUnstartedServer(gateway.Handler())
	gatewayServer.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{gatewayCertificate}}
	gatewayServer.StartTLS()
	defer gatewayServer.Close()

	rendezvous, err := node.StartRendezvous(node.RendezvousConfig{ListenAddress: rendezvousAddress, CarrierProfile: carrier, Certificate: rendezvousCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: rendezvousID, NodePublicKey: rendezvousPublic, Epoch: 10, NotAfter: deadline,
		Peers: []node.RendezvousPeer{{NodeID: initiatorID, PublicKey: initiatorPublic, Role: route.InitiatorRole},
			{NodeID: responderID, PublicKey: responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer rendezvous.Close()
	initiator, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: initiatorAddress, Certificate: initiatorCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: initiatorID, NodePublicKey: initiatorPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:        node.InitiatorPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress, CarrierProfile: carrier},
		ResolutionGateway: node.ResolutionGateway{NodeID: gatewayID, PublicKey: gatewayPublic, URL: gatewayServer.URL},
		Admit: c2EntryAdmit(presentation, network, digest, initiatorID, map[[32]byte]time.Time{
			serviceAttachment: deadline, resolutionAttachment: resolutionDeadline}),
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := node.StartResponder(node.ResponderConfig{ListenAddress: responderAddress, Certificate: responderCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: responderID, NodePublicKey: responderPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:     node.ResponderPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress, CarrierProfile: carrier},
		Admit:          c2ResponderAdmit(responderAuthorization, serviceAttachment, network, digest, responderID, deadline),
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	introduction, err := node.StartIntroduction(node.IntroductionConfig{ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 10, NotAfter: deadline,
		Admit: c2IntroductionAdmit(network, digest, introductionID, deadline), HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	publisher, current, hpkePrivate, instancePrivate := c2PublishedEndpoint(t, network, now)
	defer publisher.Close()
	alphaBinding := c2PrivateAlphaBinding(t, network, current.Credential.Target, now)
	publisherSlot, err := publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: 10,
			Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
			Rendezvous:       endpointapi.TransitPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
			Responder:        endpointapi.TransitPeer{NodeID: responderID, PublicKey: responderPublic, Endpoint: responderAddress},
			SlotAttachmentID: slotAttachment, Reachability: slotReachability, JoinHandle: join, NotAfter: deadline,
			SlotAuthorization: slotAuthorization, ResponderAuthorization: responderAuthorization}, HPKEPrivate: hpkePrivate, At: now})
	if err != nil {
		t.Fatal(err)
	}
	defer publisherSlot.Close()
	descriptor, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: instancePrivate,
		Introduction: reachability.Introduction{StateDigest: digest, Epoch: 10, IntroductionNodeID: introductionID,
			RendezvousNodeID: rendezvousID, Reachability: slotReachability, JoinHandle: join, NotAfter: deadline,
			SubmissionAuthorization: join[:]}})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := gateway.Publish(descriptor, now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("Gateway Publish = %+v, %v", result, err)
	}

	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: c2Identifier(72),
		AuthorityPublic: current.Credential.AuthorityPublic[:], IntroductionPublic: make([]byte, 32), ConnectionPrincipal: c2Identifier(73)})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	publisherApplication, serviceApplication := net.Pipe()
	defer serviceApplication.Close()
	publisherSession, err := publisher.Admit(c2Identifier(41), broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	publisherDone := make(chan serviceOutcome, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		result, runErr := publisherSlot.Accept(ctx, endpointapi.InboundConnectionRequest{Principal: c2Identifier(41), Capability: publisherSession,
			Application: publisherApplication, BytesEachDirection: 64 << 10, At: now})
		publisherDone <- serviceOutcome{result, runErr}
	}()
	requestSeen := make(chan *http.Request, 1)
	go serveOneStaticReference(serviceApplication, requestSeen)
	userSession, err := user.Admit(c2Identifier(73), broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	session := user.StartAlphaUserReferenceSite(ctx, endpointapi.AlphaUserReferenceSiteRequest{Binding: alphaBinding,
		Route: endpointapi.UserReferenceSiteRequest{Reachability: &endpointapi.UserReachabilityRouteRequest{
			Private: &endpointapi.UserPrivateReachabilityRequest{GatewayNodeID: gatewayID, GatewayNodePublicKey: gatewayPublic, GatewayFamily: c2Identifier(109),
				GatewayProfile: gateway.Profile(), StateDigest: digest, Epoch: 10,
				Initiator:    endpointapi.TransitPeer{NodeID: initiatorID, PublicKey: initiatorPublic, Endpoint: initiatorAddress},
				Entry:        c2EntryAcquirer{candidate: entry.Candidate{NodeID: initiatorID, PublicKey: initiatorPublic, Endpoint: initiatorAddress}, presentation: presentation},
				AttachmentID: resolutionAttachment, At: now, Deadline: resolutionDeadline},
			Introduction: endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Family: c2Identifier(63), Endpoint: introductionAddress},
			Entry:        c2EntryAcquirer{candidate: entry.Candidate{NodeID: initiatorID, PublicKey: initiatorPublic, Endpoint: initiatorAddress}, presentation: presentation},
			Initiator:    endpointapi.TransitPeer{NodeID: initiatorID, PublicKey: initiatorPublic, Family: c2Identifier(66), Endpoint: initiatorAddress},
			Rendezvous:   endpointapi.TransitPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Family: c2Identifier(64), Endpoint: rendezvousAddress},
			AttachmentID: serviceAttachment, EndpointHandshake: c2Identifier(74), At: now},
			Routes: map[string]string{"": "/"}, Principal: c2Identifier(73), Capability: userSession, BytesEachDirection: 64 << 10}})
	defer session.Close()
	if event := <-session.Events(); event.State != endpointapi.UserReferenceStarting {
		t.Fatalf("first User Reference event = %+v", event)
	}
	readyEvent := <-session.Events()
	readyReference := readyEvent.Ready
	if readyEvent.State != endpointapi.UserReferenceReady || readyReference.AuthenticatedTarget != current.Credential.Target ||
		readyReference.URL != "http://reference.ard/" || readyReference.AlphaProxyURL == "" {
		t.Fatalf("Reference Site was not opened after exact C2 target authentication: %+v", readyEvent)
	}
	proxyURL, parseErr := url.Parse(readyReference.AlphaProxyURL)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := httpClient.Get(readyReference.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" {
		t.Fatalf("Reference response = %d %q %v", response.StatusCode, body, readErr)
	}
	if request := <-requestSeen; request == nil || request.URL.Path != "/" || request.Host != "reference" {
		t.Fatalf("Publisher saw invalid Reference request: %#v", request)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case event, open := <-session.Events():
		if !open || event.State != endpointapi.UserReferenceStopped || event.Class == "" {
			t.Fatalf("User Endpoint terminal lifecycle event = %+v (open=%t)", event, open)
		}
	case <-time.After(time.Second):
		t.Fatal("User Endpoint did not close its scoped Reference origin")
	}
	if _, open := <-session.Events(); open {
		t.Fatal("User Endpoint lifecycle did not terminate after its terminal event")
	}
	select {
	case outcome := <-publisherDone:
		if outcome.result.Class == "" {
			t.Fatalf("Publisher Endpoint produced no classified terminal result: %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("Publisher Endpoint did not terminate after Reference close")
	}
	drain, drainCancel := context.WithTimeout(context.Background(), time.Second)
	defer drainCancel()
	if err := introduction.Drain(drain); err != nil {
		t.Fatal(err)
	}
	if err := responder.Drain(drain); err != nil {
		t.Fatal(err)
	}
	if err := initiator.Drain(drain); err != nil {
		t.Fatal(err)
	}
	if err := rendezvous.Drain(drain); err != nil {
		t.Fatal(err)
	}
	if usage := initiator.Usage(); usage.CompletedRelays != 2 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Initiator terminal usage = %+v", usage)
	}
	if usage := responder.Usage(); usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Responder terminal usage = %+v", usage)
	}
	if usage := rendezvous.Usage(); usage.CompletedPairs != 1 || usage.ActivePairs != 0 || usage.Connections != 0 {
		t.Fatalf("Rendezvous terminal usage = %+v", usage)
	}
}

func c2CarrierAddress(t *testing.T, carrier route.CarrierProfile) string {
	if carrier == route.CarrierTCP {
		return c2AvailableAddress(t)
	}
	connection, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := connection.LocalAddr().String()
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func TestUserIntroductionRouteRejectsRendezvousSubstitutionBeforeEntry(t *testing.T) {
	fixture := newFixture(t)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: c2Identifier(76),
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic, ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	entry := &c2CountingEntry{}
	_, err = user.OpenUserIntroductionRoute(context.Background(), endpointapi.UserIntroductionRouteRequest{
		Introduction: endpointapi.UserIntroductionProfile{NetworkID: fixture.networkID, Digest: c2Identifier(77), Epoch: 1,
			Introduction:     endpointapi.TransitPeer{NodeID: c2Identifier(78), PublicKey: c2Identifier(79), Endpoint: "127.0.0.1:37079"},
			RendezvousNodeID: c2Identifier(80), Reachability: c2Identifier(81), JoinHandle: c2Identifier(82),
			NotAfter: fixture.now.Add(time.Minute), SubmissionAuthorization: []byte("introduction")},
		Entry: entry, Initiator: endpointapi.TransitPeer{NodeID: c2Identifier(83), PublicKey: c2Identifier(84), Endpoint: "127.0.0.1:37084"},
		Rendezvous:   endpointapi.TransitPeer{NodeID: c2Identifier(85), PublicKey: c2Identifier(86), Endpoint: "127.0.0.1:37086"},
		AttachmentID: c2Identifier(87), EndpointHandshake: c2Identifier(88), At: fixture.now})
	if err == nil || entry.calls != 0 {
		t.Fatalf("Rendezvous substitution opened Entry: calls=%d err=%v", entry.calls, err)
	}
}

func TestUserReachabilityRouteRejectsPrivateLookupC2OverlapBeforeEntry(t *testing.T) {
	fixture := newFixture(t)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: c2Identifier(112),
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic, ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	entry := &c2CountingEntry{}
	peer := func(value byte) endpointapi.TransitPeer {
		return endpointapi.TransitPeer{NodeID: c2Identifier(value), PublicKey: c2Identifier(value + 1), Family: c2Identifier(value + 2), Endpoint: "127.0.0.1:37112"}
	}
	base := endpointapi.UserReachabilityRouteRequest{TargetLink: "not-an-ardents-target-link", Entry: entry,
		Introduction: peer(113), Initiator: peer(116), Rendezvous: peer(119), AttachmentID: c2Identifier(122), EndpointHandshake: c2Identifier(123), At: fixture.now}
	for _, test := range []struct {
		name    string
		private endpointapi.UserPrivateReachabilityRequest
	}{
		{name: "reused attachment", private: endpointapi.UserPrivateReachabilityRequest{AttachmentID: base.AttachmentID}},
		{name: "gateway identity", private: endpointapi.UserPrivateReachabilityRequest{AttachmentID: c2Identifier(124), GatewayNodeID: base.Introduction.NodeID}},
		{name: "gateway family", private: endpointapi.UserPrivateReachabilityRequest{AttachmentID: c2Identifier(125), GatewayFamily: base.Rendezvous.Family}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.Private = &test.private
			if _, routeErr := user.OpenUserReachabilityRoute(context.Background(), input); routeErr == nil {
				t.Fatal("private lookup/C2 overlap was accepted")
			}
			if entry.calls != 0 {
				t.Fatalf("private lookup/C2 overlap opened Entry: %d", entry.calls)
			}
		})
	}
}

func TestUserReferenceSessionReportsUnavailableForInvalidInput(t *testing.T) {
	fixture := newFixture(t)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: c2Identifier(101),
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic, ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	session := user.StartUserReferenceSite(t.Context(), endpointapi.UserReferenceSiteRequest{})
	first, open := <-session.Events()
	if !open || first.State != endpointapi.UserReferenceStarting {
		t.Fatalf("first invalid-input session event = %+v (open=%t)", first, open)
	}
	last, open := <-session.Events()
	if !open || last.State != endpointapi.UserReferenceUnavailable || last.Class != "service unavailable" || last.Reason == "" {
		t.Fatalf("invalid-input session event = %+v (open=%t)", last, open)
	}
	if _, open := <-session.Events(); open {
		t.Fatal("invalid-input User Reference session remained open")
	}
}

func TestAlphaUserReferenceSiteRejectsSuppliedTargetLinkBeforeC2(t *testing.T) {
	fixture := newFixture(t)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: c2Identifier(126),
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic, ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	entry := &c2CountingEntry{}
	_, err = user.OpenAlphaUserReferenceSite(t.Context(), endpointapi.AlphaUserReferenceSiteRequest{
		Binding: c2IssuedAlphaBinding(t, fixture.networkID, c2Identifier(127), fixture.now),
		Route: endpointapi.UserReferenceSiteRequest{Reachability: &endpointapi.UserReachabilityRouteRequest{
			TargetLink: "caller-supplied-target", Entry: entry}},
	})
	if err == nil || entry.calls != 0 {
		t.Fatalf("alpha route accepted a caller Target Link or opened Entry: calls=%d err=%v", entry.calls, err)
	}
}

func TestUserIntroductionRouteRejectsInvalidTargetLinkBeforeEntry(t *testing.T) {
	fixture := newFixture(t)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: c2Identifier(89),
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic, ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	entry := &c2CountingEntry{}
	now := time.Now().UTC().Truncate(time.Second)
	rendezvous := endpointapi.TransitPeer{NodeID: c2Identifier(90), PublicKey: c2Identifier(91), Endpoint: "127.0.0.1:37091"}
	_, err = user.OpenUserIntroductionRoute(context.Background(), endpointapi.UserIntroductionRouteRequest{
		TargetLink: "not-an-ardents-target-link",
		Introduction: endpointapi.UserIntroductionProfile{NetworkID: fixture.networkID, Digest: c2Identifier(92), Epoch: 1,
			Introduction:     endpointapi.TransitPeer{NodeID: c2Identifier(93), PublicKey: c2Identifier(94), Endpoint: "127.0.0.1:37094"},
			RendezvousNodeID: rendezvous.NodeID, Reachability: c2Identifier(95), JoinHandle: c2Identifier(96),
			NotAfter: now.Add(time.Minute), SubmissionAuthorization: []byte("introduction")},
		Entry: entry, Initiator: endpointapi.TransitPeer{NodeID: c2Identifier(97), PublicKey: c2Identifier(98), Endpoint: "127.0.0.1:37098"},
		Rendezvous: rendezvous, AttachmentID: c2Identifier(99), EndpointHandshake: c2Identifier(100), At: now})
	if err == nil || entry.calls != 0 {
		t.Fatalf("invalid Target Link opened Entry: calls=%d err=%v", entry.calls, err)
	}
}

func c2EntryAdmit(presentation entry.Presentation, network, digest, initiator [32]byte, attachments map[[32]byte]time.Time) route.EntryBindingAdmitter {
	return func(invite []byte, received, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		deadline, exists := attachments[received]
		if string(invite) != string(presentation.Invite) || !exists || key == [32]byte{} || !notAfter.Equal(deadline) {
			return route.EntryAdmission{}, errors.New("unexpected C2 Entry admission")
		}
		return route.EntryAdmission{InviteID: presentation.InviteID, NetworkID: network, Digest: digest, Epoch: 10,
			InitiatorNodeID: initiator, NotAfter: deadline}, nil
	}
}

func c2GatewayCertificate(t *testing.T) (tls.Certificate, [32]byte, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(113), Subject: pkix.Name{CommonName: "destination-resolution"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var identifier [32]byte
	copy(identifier[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identifier, private
}

func c2ResponderAdmit(authorization []byte, attachment, network, digest, responder [32]byte, deadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(received []byte, gotAttachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if string(received) != string(authorization) || gotAttachment != attachment || key == [32]byte{} || role != route.ResponderRole || nodeID != responder || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected C2 Responder admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(75), NetworkID: network, Digest: digest, Epoch: 10,
			TransitRole: route.ResponderRole, TransitNodeID: responder, NotAfter: deadline}, nil
	}
}

type c2EntryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

type c2CountingEntry struct{ calls int }

func (acquirer *c2CountingEntry) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	acquirer.calls++
	return nil, nil, errors.New("Entry must not be opened")
}

func (input c2EntryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}
