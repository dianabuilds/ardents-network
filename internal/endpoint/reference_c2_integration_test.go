package endpoint_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestC2RouteCarriesReferenceSiteBetweenTwoEndpoints(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network, digest := c2Identifier(61), c2Identifier(62)
	introductionID, rendezvousID := c2Identifier(63), c2Identifier(64)
	responderID, initiatorID := c2Identifier(65), c2Identifier(66)
	introductionCertificate, introductionPublic := c2Certificate(t, 61, "introduction")
	rendezvousCertificate, rendezvousPublic := c2Certificate(t, 62, "rendezvous")
	responderCertificate, responderPublic := c2Certificate(t, 63, "responder")
	initiatorCertificate, initiatorPublic := c2Certificate(t, 64, "initiator")
	introductionAddress, rendezvousAddress := c2AvailableAddress(t), c2AvailableAddress(t)
	responderAddress, initiatorAddress := c2AvailableAddress(t), c2AvailableAddress(t)
	join, reachability := c2Identifier(67), c2Identifier(68)
	slotAttachment, serviceAttachment := c2Identifier(69), c2Identifier(70)
	slotAuthorization, responderAuthorization := []byte("publisher-slot"), []byte("publisher-responder")
	presentation := entry.Presentation{InviteID: c2Identifier(71), Invite: []byte("entry-invite")}

	rendezvous, err := node.StartRendezvous(node.RendezvousConfig{ListenAddress: rendezvousAddress, Certificate: rendezvousCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: rendezvousID, NodePublicKey: rendezvousPublic, Epoch: 10, NotAfter: deadline,
		Peers: []node.RendezvousPeer{{NodeID: initiatorID, PublicKey: initiatorPublic, Role: route.InitiatorRole},
			{NodeID: responderID, PublicKey: responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer rendezvous.Close()
	initiator, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: initiatorAddress, Certificate: initiatorCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: initiatorID, NodePublicKey: initiatorPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:     node.InitiatorPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
		Admit:          c2EntryAdmit(presentation, serviceAttachment, network, digest, initiatorID, deadline),
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := node.StartResponder(node.ResponderConfig{ListenAddress: responderAddress, Certificate: responderCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: responderID, NodePublicKey: responderPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:     node.ResponderPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
		Admit:          c2ResponderAdmit(responderAuthorization, serviceAttachment, network, digest, responderID, deadline),
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	introduction, err := node.StartIntroduction(node.IntroductionConfig{ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 10, NotAfter: deadline,
		Admit: c2IntroductionAdmit(network, digest, introductionID, deadline), HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	publisher, current, hpkePrivate := c2PublishedEndpoint(t, network, now)
	defer publisher.Close()
	link, err := targetlink.Encode(targetlink.Link{Network: network, Target: current.Credential.Target})
	if err != nil {
		t.Fatal(err)
	}
	publisherSlot, err := publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: 10,
			Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
			Rendezvous:       endpointapi.TransitPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
			Responder:        endpointapi.TransitPeer{NodeID: responderID, PublicKey: responderPublic, Endpoint: responderAddress},
			SlotAttachmentID: slotAttachment, Reachability: reachability, JoinHandle: join, NotAfter: deadline,
			SlotAuthorization: slotAuthorization, ResponderAuthorization: responderAuthorization}, HPKEPrivate: hpkePrivate, At: now})
	if err != nil {
		t.Fatal(err)
	}
	defer publisherSlot.Close()

	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: c2Identifier(72),
		AuthorityPublic: current.Credential.AuthorityPublic[:], IntroductionPublic: make([]byte, 32), ConnectionPrincipal: c2Identifier(73)})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	userRoute, cleanup, err := route.OpenEntryAttachment(context.Background(), c2EntryAcquirer{
		candidate: entry.Candidate{NodeID: initiatorID, PublicKey: initiatorPublic, Endpoint: initiatorAddress}, presentation: presentation},
		route.EntryAttachmentRequest{NetworkID: network, Digest: digest, Epoch: 10, AttachmentID: serviceAttachment, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	setup := route.RelaySetup{NetworkID: network, Digest: digest, AttachmentID: serviceAttachment, Epoch: 10,
		TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole, TransitNodeID: initiatorID,
		NextNodeID: rendezvousID, NextNodePublicKey: rendezvousPublic, NotAfter: deadline}
	if err := route.WriteRelaySetup(userRoute, setup); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadRelayReady(userRoute)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		t.Fatalf("Initiator RelayReady = %+v, %v", ready, err)
	}

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
	delivery, err := user.SubmitIntroductionFromLink(ctx, endpointapi.UserIntroductionRequest{TargetLink: link, Publication: current.Record,
		Profile: endpointapi.UserIntroductionProfile{NetworkID: network, Digest: digest, Epoch: 10,
			Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
			RendezvousNodeID: rendezvousID, Reachability: reachability, JoinHandle: join, NotAfter: deadline, SubmissionAuthorization: join[:]},
		AttachmentID: serviceAttachment, EndpointHandshake: c2Identifier(74), At: now})
	if err != nil || delivery.AuthenticatedTarget != current.Credential.Target || delivery.AttachmentID != serviceAttachment {
		t.Fatalf("C2 delivery = %+v, %v", delivery, err)
	}
	requestSeen := make(chan *http.Request, 1)
	go serveOneStaticReference(serviceApplication, requestSeen)
	userSession, err := user.Admit(c2Identifier(73), broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	running, err := user.StartReferenceConnection(ctx, endpointapi.ReferenceConnectionRequest{TargetLink: link, Routes: map[string]string{"": "/"},
		Connection: endpointapi.OutboundConnectionRequest{Principal: c2Identifier(73), Capability: userSession, Target: current.Credential.Target,
			Publication: current.Record, Route: userRoute, BytesEachDirection: 64 << 10, At: now}})
	if err != nil {
		t.Fatal(err)
	}
	readyReference, ok := <-running.Ready()
	if !ok || readyReference.AuthenticatedTarget != current.Credential.Target {
		t.Fatalf("Reference Site was not opened after exact C2 target authentication: %+v", readyReference)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: nil}}
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
	_ = running.Close()
	select {
	case outcome := <-running.Done():
		if outcome.Result.Class == "" {
			t.Fatalf("User Endpoint produced no classified terminal result: %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("User Endpoint did not close its scoped Reference origin")
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
	if usage := initiator.Usage(); usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Initiator terminal usage = %+v", usage)
	}
	if usage := responder.Usage(); usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
		t.Fatalf("Responder terminal usage = %+v", usage)
	}
	if usage := rendezvous.Usage(); usage.CompletedPairs != 1 || usage.ActivePairs != 0 || usage.Connections != 0 {
		t.Fatalf("Rendezvous terminal usage = %+v", usage)
	}
}

func c2EntryAdmit(presentation entry.Presentation, attachment, network, digest, initiator [32]byte, deadline time.Time) route.EntryBindingAdmitter {
	return func(invite []byte, received, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		if string(invite) != string(presentation.Invite) || received != attachment || key == [32]byte{} || !notAfter.Equal(deadline) {
			return route.EntryAdmission{}, errors.New("unexpected C2 Entry admission")
		}
		return route.EntryAdmission{InviteID: presentation.InviteID, NetworkID: network, Digest: digest, Epoch: 10,
			InitiatorNodeID: initiator, NotAfter: deadline}, nil
	}
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

func (input c2EntryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}
