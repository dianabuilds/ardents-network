//go:build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

var (
	rendezvousNode         = identifier("rendezvous-node")
	initiatorNode          = identifier("initiator-node")
	responderNode          = identifier("responder-node")
	responderAuthorization = identifier("responder-authorization")
)

func fullEndpoints() (introduction, initiator, rendezvous, responder string) {
	return fmt.Sprintf("127.0.0.1:%d", tracerBasePort), fmt.Sprintf("127.0.0.1:%d", tracerBasePort+1),
		fmt.Sprintf("127.0.0.1:%d", tracerBasePort+2), fmt.Sprintf("127.0.0.1:%d", tracerBasePort+3)
}

func runFullRendezvous(ctx context.Context, deadline time.Time) (result, error) {
	_, _, endpoint, _ := fullEndpoints()
	material, err := material()
	if err != nil {
		return result{}, err
	}
	running, err := node.StartRendezvous(node.RendezvousConfig{ListenAddress: endpoint, Certificate: material.rendezvous,
		NetworkID: networkID, EpochDigest: epochDigest, NodeID: rendezvousNode, NodePublicKey: material.rendezvousPublic, Epoch: epoch,
		NotAfter: deadline, Peers: []node.RendezvousPeer{{NodeID: initiatorNode, PublicKey: material.initiatorPublic, Role: route.InitiatorRole},
			{NodeID: responderNode, PublicKey: material.responderPublic, Role: route.ResponderRole}}, HandshakeLimit: 2, WaitingLimit: 2,
		PairLimit: 1, PairByteLimit: 4096, DrainTimeout: time.Second})
	if err != nil {
		return result{}, err
	}
	fmt.Println(`{"event":"ready"}`)
	for {
		if running.Usage().CompletedPairs == 1 {
			break
		}
		select {
		case <-ctx.Done():
			return result{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := running.Drain(context.Background()); err != nil {
		return result{}, err
	}
	usage := running.Usage()
	return result{Schema: "ardents-r105-rendezvous-v1", Role: "rendezvous", Mode: "full", Passed: usage.Connections == 0 && usage.ActivePairs == 0, Delivered: int(usage.CompletedPairs)}, nil
}

func runFullInitiator(ctx context.Context, deadline time.Time) (result, error) {
	_, endpoint, rendezvousEndpoint, _ := fullEndpoints()
	material, err := material()
	if err != nil {
		return result{}, err
	}
	invite := []byte("r105-full-entry-invite")
	running, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: endpoint, Certificate: material.initiator,
		NetworkID: networkID, EpochDigest: epochDigest, NodeID: initiatorNode, NodePublicKey: material.initiatorPublic, Epoch: epoch,
		NotAfter: deadline, Rendezvous: node.InitiatorPeer{NodeID: rendezvousNode, PublicKey: material.rendezvousPublic, Endpoint: rendezvousEndpoint},
		Admit: func(received []byte, attachment, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
			if string(received) != string(invite) || attachment != attachmentID || key == [32]byte{} || !notAfter.Equal(deadline) {
				return route.EntryAdmission{}, errors.New("full Entry admission mismatch")
			}
			return route.EntryAdmission{InviteID: identifier("entry-invite"), NetworkID: networkID, Digest: epochDigest, Epoch: epoch, InitiatorNodeID: initiatorNode, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 4096, DrainTimeout: time.Second})
	if err != nil {
		return result{}, err
	}
	fmt.Println(`{"event":"ready"}`)
	for {
		if running.Usage().CompletedRelays == 1 {
			break
		}
		select {
		case <-ctx.Done():
			return result{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := running.Drain(context.Background()); err != nil {
		return result{}, err
	}
	usage := running.Usage()
	return result{Schema: "ardents-r105-initiator-v1", Role: "initiator", Mode: "full", Passed: usage.Connections == 0 && usage.ActiveRelays == 0, Delivered: int(usage.CompletedRelays)}, nil
}

func runFullResponder(ctx context.Context, deadline time.Time) (result, error) {
	_, _, rendezvousEndpoint, endpoint := fullEndpoints()
	material, err := material()
	if err != nil {
		return result{}, err
	}
	running, err := node.StartResponder(node.ResponderConfig{ListenAddress: endpoint, Certificate: material.responder, NetworkID: networkID,
		EpochDigest: epochDigest, NodeID: responderNode, NodePublicKey: material.responderPublic, Epoch: epoch, NotAfter: deadline,
		Rendezvous: node.ResponderPeer{NodeID: rendezvousNode, PublicKey: material.rendezvousPublic, Endpoint: rendezvousEndpoint},
		Admit: func(auth []byte, receivedAttachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
			if string(auth) != string(responderAuthorization[:]) || receivedAttachment != attachmentID || key == [32]byte{} || role != route.ResponderRole || nodeID != responderNode || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("full Responder admission mismatch")
			}
			return route.EndpointTransitAdmission{AuthorizationID: identifier("responder-admission"), NetworkID: networkID, Digest: epochDigest, Epoch: epoch, TransitRole: route.ResponderRole, TransitNodeID: responderNode, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 4096, DrainTimeout: time.Second})
	if err != nil {
		return result{}, err
	}
	fmt.Println(`{"event":"ready"}`)
	for running.Usage().CompletedRelays != 1 {
		select {
		case <-ctx.Done():
			return result{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := running.Drain(context.Background()); err != nil {
		return result{}, err
	}
	usage := running.Usage()
	return result{Schema: "ardents-r105-responder-v1", Role: "responder", Mode: "full", Passed: usage.Connections == 0 && usage.ActiveRelays == 0, Delivered: int(usage.CompletedRelays)}, nil
}

func runFullPublisher(ctx context.Context, introductionEndpoint string, deadline time.Time) (result, error) {
	material, err := material()
	if err != nil {
		return result{}, err
	}
	connection, err := openPublisherSlot(ctx, introductionEndpoint, deadline, material)
	if err != nil {
		return result{}, err
	}
	defer connection.Close()
	fmt.Println(`{"event":"slot-ready"}`)
	record, err := route.ReadIntroductionControlRecord(connection)
	if err != nil || record.Sealed == nil {
		return result{}, errors.New("full Publisher received no sealed Introduction")
	}
	plaintext, err := route.OpenSealedIntroduction(*record.Sealed, material.hpkePrivate)
	if err != nil {
		return result{}, err
	}
	instruction, err := publication.DecodeIntroductionInstruction(plaintext)
	if err != nil {
		return result{}, err
	}
	current := publication.Current{Credential: publication.Credential{Target: target, Generation: 1}, Digest: publicationDigest}
	if err := current.ValidateIntroductionInstruction(instruction); err != nil || instruction.AttachmentID != attachmentID {
		return result{}, errors.New("full Publisher rejected Service Introduction")
	}
	_, _, _, responderEndpoint := fullEndpoints()
	carrier, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{NetworkID: networkID, Digest: epochDigest, AttachmentID: attachmentID,
		TransitNodeID: responderNode, TransitNodePublicKey: material.responderPublic, Epoch: epoch, TransitRole: route.ResponderRole,
		Endpoint: responderEndpoint, Deadline: deadline, Authorization: responderAuthorization[:]})
	if err != nil {
		return result{}, err
	}
	defer carrier.Close()
	request := make([]byte, len("GET / HTTP/1.1\r\nHost: reference\r\n\r\n"))
	if _, err := io.ReadFull(carrier, request); err != nil {
		return result{}, err
	}
	const page = "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 29\r\n\r\n<h1>Ardents Reference</h1>"
	if _, err := carrier.Write([]byte(page)); err != nil {
		return result{}, err
	}
	return result{Schema: "ardents-r105-publisher-v1", Role: "publisher", Mode: "full", Passed: true, Delivered: 1}, nil
}

func runFullUser(ctx context.Context, introductionEndpoint string, deadline time.Time) (result, error) {
	if _, err := runUser(ctx, introductionEndpoint, deadline, "full"); err != nil {
		return result{}, err
	}
	_, initiatorEndpoint, _, _ := fullEndpoints()
	material, err := material()
	if err != nil {
		return result{}, err
	}
	candidate := entry.Candidate{NodeID: initiatorNode, PublicKey: material.initiatorPublic, Endpoint: initiatorEndpoint}
	connection, cleanup, err := route.OpenEntryAttachment(ctx, fixedAcquirer{candidate: candidate, presentation: entry.Presentation{InviteID: identifier("entry-invite"), Invite: []byte("r105-full-entry-invite")}}, route.EntryAttachmentRequest{NetworkID: networkID, Digest: epochDigest, Epoch: epoch, AttachmentID: attachmentID, Deadline: deadline})
	if err != nil {
		return result{}, err
	}
	defer cleanup()
	setup := route.RelaySetup{NetworkID: networkID, Digest: epochDigest, AttachmentID: attachmentID, Epoch: epoch, TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole,
		TransitNodeID: initiatorNode, NextNodeID: rendezvousNode, NextNodePublicKey: material.rendezvousPublic, NotAfter: deadline}
	if err := route.WriteRelaySetup(connection, setup); err != nil {
		return result{}, err
	}
	ready, err := route.ReadRelayReady(connection)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		return result{}, errors.New("full User did not receive exact RelayReady")
	}
	request := []byte("GET / HTTP/1.1\r\nHost: reference\r\n\r\n")
	if _, err := connection.Write(request); err != nil {
		return result{}, err
	}
	want := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 29\r\n\r\n<h1>Ardents Reference</h1>")
	response := make([]byte, len(want))
	if _, err := io.ReadFull(connection, response); err != nil || string(response) != string(want) {
		return result{}, errors.New("full User did not receive Reference Site bytes")
	}
	return result{Schema: "ardents-r105-user-v1", Role: "user", Mode: "full", Passed: true, Delivered: 1}, nil
}

func openPublisherSlot(ctx context.Context, endpoint string, deadline time.Time, material tracerMaterial) (net.Conn, error) {
	connection, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{NetworkID: networkID, Digest: epochDigest,
		AttachmentID: identifier("publisher-slot-attachment"), TransitNodeID: introductionNode, TransitNodePublicKey: material.introductionPublic,
		Epoch: epoch, TransitRole: route.IntroductionRole, Endpoint: endpoint, Deadline: deadline, Authorization: publisherAuthorization[:]})
	if err != nil {
		return nil, err
	}
	if err := route.WriteIntroductionSlotRegistration(connection, route.IntroductionSlotRegistration{Reachability: reachability, JoinHandle: joinHandle, NotAfter: deadline}); err != nil {
		_ = connection.Close()
		return nil, err
	}
	ready, err := route.ReadIntroductionSlotReady(connection)
	if err != nil || ready.Reachability != reachability || ready.JoinHandle != joinHandle || !ready.NotAfter.Equal(deadline) {
		_ = connection.Close()
		return nil, errors.New("full Publisher slot was not registered")
	}
	return connection, nil
}

type fixedAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input fixedAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}

func relayPair(first, second net.Conn) error {
	results := make(chan error, 2)
	copyLane := func(destination, source net.Conn) { _, err := io.Copy(destination, source); results <- err }
	go copyLane(first, second)
	go copyLane(second, first)
	if err := <-results; err != nil && !errors.Is(err, net.ErrClosed) {
		_ = first.Close()
		_ = second.Close()
		<-results
		return err
	}
	_ = first.Close()
	_ = second.Close()
	<-results
	return nil
}
