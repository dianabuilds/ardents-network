//go:build ignore

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

type result struct {
	Schema        string `json:"schema"`
	Role          string `json:"role"`
	Mode          string `json:"mode"`
	Passed        bool   `json:"passed"`
	Delivered     int    `json:"delivered"`
	ReplayRefused int    `json:"replay_refused"`
	Reason        string `json:"reason,omitempty"`
}

var (
	networkID              = identifier("network")
	epochDigest            = identifier("epoch-digest")
	introductionNode       = identifier("introduction-node")
	reachability           = identifier("reachability")
	joinHandle             = identifier("join-handle")
	publisherAuthorization = identifier("publisher-authorization")
	attachmentID           = identifier("attachment")
	target                 = identifier("target")
	publicationDigest      = identifier("publication-digest")
)

const epoch uint64 = 7

var tracerBasePort int

func runRole(ctx context.Context, role, endpoint string, deadline time.Time, mode string) (result, error) {
	switch role {
	case "introduction":
		return runIntroduction(ctx, endpoint, deadline, mode)
	case "publisher":
		return runPublisher(ctx, endpoint, deadline, mode)
	case "user":
		return runUser(ctx, endpoint, deadline, mode)
	case "full-rendezvous":
		return runFullRendezvous(ctx, deadline)
	case "full-initiator":
		return runFullInitiator(ctx, deadline)
	case "full-responder":
		return runFullResponder(ctx, deadline)
	case "full-publisher":
		return runFullPublisher(ctx, endpoint, deadline)
	case "full-user":
		return runFullUser(ctx, endpoint, deadline)
	default:
		return result{}, errors.New("unknown tracer role")
	}
}

type introductionSlot struct {
	registration route.IntroductionSlotRegistration
	connection   net.Conn
	spent        bool
}

func runIntroduction(ctx context.Context, endpoint string, deadline time.Time, mode string) (result, error) {
	if endpoint == "" {
		return result{}, errors.New("Introduction endpoint is missing")
	}
	material, err := material()
	if err != nil {
		return result{}, err
	}
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		return result{}, err
	}
	defer listener.Close()
	fmt.Println(`{"event":"ready"}`)
	var mu sync.Mutex
	var slot *introductionSlot
	usedAuthorization := map[string]bool{}
	delivered, replayRefused, submissions := 0, 0, 0
	expectedSubmissions := 1
	if mode == "replay" {
		expectedSubmissions = 2
	}
	done := make(chan struct{})
	complete := sync.Once{}
	finish := func() { complete.Do(func() { close(done); _ = listener.Close() }) }
	admit := func(authorization []byte, receivedAttachment, key [32]byte, role byte, node [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		mu.Lock()
		defer mu.Unlock()
		if receivedAttachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || node != introductionNode ||
			!notAfter.Equal(deadline) || (!bytes.Equal(authorization, publisherAuthorization[:]) && !bytes.Equal(authorization, joinHandle[:])) || usedAuthorization[string(authorization)] {
			if bytes.Equal(authorization, joinHandle[:]) {
				replayRefused++
				submissions++
				if submissions >= expectedSubmissions {
					finish()
				}
			}
			return route.EndpointTransitAdmission{}, errors.New("synthetic C2 authorization is unavailable")
		}
		usedAuthorization[string(authorization)] = true
		return route.EndpointTransitAdmission{AuthorizationID: identifier(fmt.Sprintf("admission-%x", key)), NetworkID: networkID,
			Digest: epochDigest, Epoch: epoch, TransitRole: route.IntroductionRole, TransitNodeID: introductionNode, NotAfter: deadline}, nil
	}
	handle := func(raw net.Conn) {
		accepted, acceptErr := route.AcceptEndpointTransitAttachment(ctx, raw, route.EndpointTransitAttachmentAcceptance{NetworkID: networkID,
			Digest: epochDigest, TransitNodeID: introductionNode, Epoch: epoch, TransitRole: route.IntroductionRole, Deadline: deadline,
			Certificate: material.introduction, Admit: admit})
		if acceptErr != nil {
			return
		}
		connection := accepted.Connection
		record, readErr := route.ReadIntroductionControlRecord(connection)
		if readErr != nil {
			_ = connection.Close()
			return
		}
		if record.Registration != nil {
			mu.Lock()
			valid := bytes.Equal(accepted.Binding.Authorization, publisherAuthorization[:]) && slot == nil &&
				record.Registration.Reachability == reachability && record.Registration.JoinHandle == joinHandle && record.Registration.NotAfter.Equal(deadline)
			if valid {
				slot = &introductionSlot{registration: *record.Registration, connection: connection}
			}
			mu.Unlock()
			if !valid {
				_ = connection.Close()
				return
			}
			_ = route.WriteIntroductionSlotReady(connection, route.IntroductionSlotReady{Reachability: reachability, JoinHandle: joinHandle, NotAfter: deadline})
			go func(registered net.Conn) {
				probe := []byte{0}
				_, _ = registered.Read(probe)
				mu.Lock()
				if slot != nil && slot.connection == registered {
					slot = nil
				}
				mu.Unlock()
			}(connection)
			return
		}
		if record.Sealed == nil || !bytes.Equal(accepted.Binding.Authorization, joinHandle[:]) {
			_ = connection.Close()
			return
		}
		mu.Lock()
		submissions++
		valid := record.Sealed.NetworkID == networkID && record.Sealed.Digest == epochDigest && record.Sealed.Epoch == epoch &&
			record.Sealed.IntroductionNodeID == introductionNode && record.Sealed.Reachability == reachability &&
			record.Sealed.JoinHandle == joinHandle && record.Sealed.NotAfter.Equal(deadline) && slot != nil && !slot.spent
		if valid {
			slot.spent = true
			_, writeErr := slot.connection.Write(record.Raw)
			if writeErr == nil {
				delivered++
			} else {
				valid = false
			}
		}
		outcome := route.IntroductionUnavailable
		if valid {
			outcome = route.IntroductionDelivered
		}
		_ = route.WriteIntroductionDeliveryResult(connection, route.IntroductionDeliveryResult{AttachmentID: accepted.Binding.AttachmentID, Outcome: outcome})
		if submissions >= expectedSubmissions {
			finish()
		}
		mu.Unlock()
		_ = connection.Close()
	}
	for {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			select {
			case <-done:
				mu.Lock()
				output := result{Schema: "ardents-r105-introduction-v1", Role: "introduction", Mode: mode,
					Passed: delivered == 1 || mode == "withdrawn-slot", Delivered: delivered, ReplayRefused: replayRefused}
				mu.Unlock()
				return output, nil
			default:
				return result{}, acceptErr
			}
		}
		go handle(raw)
	}
}

func runPublisher(ctx context.Context, endpoint string, deadline time.Time, mode string) (result, error) {
	material, err := material()
	if err != nil {
		return result{}, err
	}
	connection, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{NetworkID: networkID, Digest: epochDigest,
		AttachmentID: identifier("publisher-slot-attachment"), TransitNodeID: introductionNode, TransitNodePublicKey: material.introductionPublic,
		Epoch: epoch, TransitRole: route.IntroductionRole, Endpoint: endpoint, Deadline: deadline, Authorization: publisherAuthorization[:]})
	if err != nil {
		return result{}, err
	}
	defer connection.Close()
	registration := route.IntroductionSlotRegistration{Reachability: reachability, JoinHandle: joinHandle, NotAfter: deadline}
	if err := route.WriteIntroductionSlotRegistration(connection, registration); err != nil {
		return result{}, err
	}
	ready, err := route.ReadIntroductionSlotReady(connection)
	if err != nil || ready.Reachability != reachability || ready.JoinHandle != joinHandle || !ready.NotAfter.Equal(deadline) {
		return result{}, errors.New("Publisher slot was not registered")
	}
	fmt.Println(`{"event":"slot-ready"}`)
	if mode == "withdrawn-slot" {
		return result{Schema: "ardents-r105-publisher-v1", Role: "publisher", Mode: mode, Passed: true}, nil
	}
	record, err := route.ReadIntroductionControlRecord(connection)
	if err != nil || record.Sealed == nil {
		return result{}, errors.New("Publisher did not receive a sealed Introduction")
	}
	plaintext, openErr := route.OpenSealedIntroduction(*record.Sealed, material.hpkePrivate)
	if mode == "header-tamper" || mode == "ciphertext-tamper" {
		return result{Schema: "ardents-r105-publisher-v1", Role: "publisher", Mode: mode, Passed: openErr != nil}, nil
	}
	if openErr != nil {
		return result{}, openErr
	}
	instruction, err := publication.DecodeIntroductionInstruction(plaintext)
	if err != nil {
		return result{}, err
	}
	current := publication.Current{Credential: publication.Credential{Target: target, Generation: 1}, Digest: publicationDigest}
	if err := current.ValidateIntroductionInstruction(instruction); err != nil || instruction.AttachmentID != attachmentID {
		return result{}, errors.New("Publisher accepted a foreign Service Introduction")
	}
	return result{Schema: "ardents-r105-publisher-v1", Role: "publisher", Mode: mode, Passed: true, Delivered: 1}, nil
}

func runUser(ctx context.Context, endpoint string, deadline time.Time, mode string) (result, error) {
	material, err := material()
	if err != nil {
		return result{}, err
	}
	plaintext, err := publication.EncodeIntroductionInstruction(publication.IntroductionInstruction{Target: target, Generation: 1,
		PublicationDigest: publicationDigest, AttachmentID: attachmentID})
	if err != nil {
		return result{}, err
	}
	sealed, err := route.SealIntroduction(route.SealedIntroduction{NetworkID: networkID, Digest: epochDigest, Epoch: epoch,
		IntroductionNodeID: introductionNode, RendezvousNodeID: identifier("rendezvous-node"), Reachability: reachability,
		NotAfter: deadline, JoinHandle: joinHandle, EndpointHandshake: identifier("endpoint-handshake")}, material.hpkePublic, plaintext)
	if err != nil {
		return result{}, err
	}
	if mode == "header-tamper" {
		sealed.EndpointHandshake[0]++
	}
	if mode == "ciphertext-tamper" {
		sealed.Ciphertext[0]++
	}
	attempts := 1
	if mode == "replay" {
		attempts = 2
	}
	refused := 0
	for index := 0; index < attempts; index++ {
		connection, openErr := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{NetworkID: networkID, Digest: epochDigest,
			AttachmentID: identifier(fmt.Sprintf("user-attachment-%d", index)), TransitNodeID: introductionNode,
			TransitNodePublicKey: material.introductionPublic, Epoch: epoch, TransitRole: route.IntroductionRole, Endpoint: endpoint,
			Deadline: deadline, Authorization: joinHandle[:]})
		if openErr != nil {
			refused++
			continue
		}
		writeErr := route.WriteSealedIntroduction(connection, sealed)
		outcome, readErr := route.ReadIntroductionDeliveryResult(connection)
		_ = connection.Close()
		if writeErr != nil || readErr != nil || outcome.AttachmentID != identifier(fmt.Sprintf("user-attachment-%d", index)) || outcome.Outcome != route.IntroductionDelivered {
			refused++
		}
	}
	if mode == "replay" && refused == 0 {
		return result{}, errors.New("replayed JoinHandle was not refused")
	}
	return result{Schema: "ardents-r105-user-v1", Role: "user", Mode: mode, Passed: true, ReplayRefused: refused}, nil
}
