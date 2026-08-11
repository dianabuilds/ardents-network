package nativecircuit

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const candidateProfile = "carrier-lab-c5-c2/v1"

type candidateUserPlan struct {
	Profile            string
	RunID              string
	Rendezvous         string
	Slot               handle
	IntroductionPath   []circuitHop
	DataPath           []circuitHop
	HPKEPublic         *ecdh.PublicKey
	EndpointTrust      endpointTrust
	Payload            []byte
	Stream             *streamSpec
	Attached           net.Conn
	SetupVerified      func() error
	FirstChunkVerified func() error
}

type candidateServicePlan struct {
	Profile             string
	RunID               string
	Rendezvous          string
	Slot                handle
	IntroductionPath    []circuitHop
	DataPath            []circuitHop
	HPKEPrivate         *ecdh.PrivateKey
	EndpointCertificate tls.Certificate
	Stream              *streamSpec
	Attached            net.Conn
	Registered          func() error
}

func runCandidateUser(ctx context.Context, plan candidateUserPlan) (endpointObservation, error) {
	if err := validateUserPlan(plan); err != nil {
		return endpointObservation{}, err
	}
	joinToken, err := randomCryptoHandle()
	if err != nil {
		return endpointObservation{}, err
	}
	userAttempt, err := randomCryptoHandle()
	if err != nil {
		return endpointObservation{}, err
	}
	nonce, err := randomCryptoHandle()
	if err != nil {
		return endpointObservation{}, err
	}
	data, err := dialTelescopedCircuit(ctx, plan.DataPath)
	if err != nil {
		return endpointObservation{}, err
	}
	dataOwned := true
	defer func() {
		if dataOwned {
			_ = data.Close()
		}
	}()
	if err := writeControlFrame(data, frameRendezvousRegister, rendezvousAttachment{JoinToken: joinToken, AttemptHandle: userAttempt}); err != nil {
		return endpointObservation{}, err
	}
	sealed, err := sealInvitation(plan.HPKEPublic, invitation{
		Profile: plan.Profile, RunID: plan.RunID, Rendezvous: plan.Rendezvous,
		JoinToken: joinToken, HandshakeNonce: nonce, ExpiresUnix: time.Now().Add(30 * time.Second).Unix(),
	})
	if err != nil {
		return endpointObservation{}, err
	}
	introduction, err := dialTelescopedCircuit(ctx, plan.IntroductionPath)
	if err != nil {
		return endpointObservation{}, err
	}
	if err := writeControlFrame(introduction, frameIntroductionDeliver, introductionDeliveryFrame{Slot: plan.Slot, Sealed: sealed}); err != nil {
		_ = introduction.Close()
		return endpointObservation{}, err
	}
	acknowledgement, err := readFrame(introduction)
	_ = introduction.Close()
	if err != nil {
		return endpointObservation{}, candidatePeerReadFailure(err)
	}
	if acknowledgement.Type != frameIntroductionAcknowledge || string(acknowledgement.Payload) != "accepted" {
		return endpointObservation{}, candidateContractFailure("separate Introduction Path did not acknowledge the invitation")
	}
	joined, err := readFrame(data)
	if err != nil {
		return endpointObservation{}, candidatePeerReadFailure(err)
	}
	if joined.Type != frameRendezvousResult || string(joined.Payload) != "joined" {
		return endpointObservation{}, candidateContractFailure("rendezvous did not join the C-5 legs")
	}
	dataOwned = false
	if plan.Attached != nil {
		return runEndpointUserAttached(ctx, data, plan.EndpointTrust, nonce, plan.Attached, plan.SetupVerified)
	}
	if plan.Stream != nil {
		return runEndpointUserStream(ctx, data, plan.EndpointTrust, nonce, *plan.Stream, plan.SetupVerified)
	}
	return runEndpointUserWithCallbacks(ctx, data, plan.EndpointTrust, nonce, plan.Payload, plan.SetupVerified, plan.FirstChunkVerified)
}

func runCandidateService(ctx context.Context, plan candidateServicePlan) (endpointObservation, error) {
	if err := validateServicePlan(plan); err != nil {
		return endpointObservation{}, err
	}
	introduction, err := dialTelescopedCircuit(ctx, plan.IntroductionPath)
	if err != nil {
		return endpointObservation{}, err
	}
	defer introduction.Close()
	if err := writeControlFrame(introduction, frameIntroductionRegister, introductionRegistrationFrame{Slot: plan.Slot}); err != nil {
		return endpointObservation{}, err
	}
	registered, err := readFrame(introduction)
	if err != nil {
		return endpointObservation{}, candidatePeerReadFailure(err)
	}
	if registered.Type != frameIntroductionAcknowledge || string(registered.Payload) != "registered" {
		return endpointObservation{}, candidateContractFailure("introduction slot was not registered")
	}
	if plan.Registered != nil {
		if err := plan.Registered(); err != nil {
			return endpointObservation{}, err
		}
	}
	delivery, err := readFrame(introduction)
	if err != nil {
		return endpointObservation{}, candidatePeerReadFailure(err)
	}
	if delivery.Type != frameIntroductionDeliver {
		return endpointObservation{}, candidateContractFailure("service did not receive a sealed Introduction invitation")
	}
	guard := newInvitationGuard(plan.Profile, plan.RunID, plan.Rendezvous, time.Now())
	opened, err := guard.open(plan.HPKEPrivate, delivery.Payload)
	if err != nil {
		return endpointObservation{}, errors.Join(errCandidateContractFailure, err)
	}
	if err := writeFrame(introduction, frame{Type: frameIntroductionAcknowledge, Payload: []byte("accepted")}); err != nil {
		return endpointObservation{}, err
	}
	serviceAttempt, err := randomCryptoHandle()
	if err != nil {
		return endpointObservation{}, err
	}
	data, err := dialTelescopedCircuit(ctx, plan.DataPath)
	if err != nil {
		return endpointObservation{}, err
	}
	dataOwned := true
	defer func() {
		if dataOwned {
			_ = data.Close()
		}
	}()
	if err := writeControlFrame(data, frameRendezvousAttach, rendezvousAttachment{JoinToken: opened.JoinToken, AttemptHandle: serviceAttempt}); err != nil {
		return endpointObservation{}, err
	}
	joined, err := readFrame(data)
	if err != nil {
		return endpointObservation{}, candidatePeerReadFailure(err)
	}
	if joined.Type != frameRendezvousResult || string(joined.Payload) != "joined" {
		return endpointObservation{}, candidateContractFailure("service leg was not joined at Rendezvous")
	}
	dataOwned = false
	if plan.Attached != nil {
		return runEndpointServiceAttached(ctx, data, plan.EndpointCertificate, opened.HandshakeNonce, plan.Attached)
	}
	if plan.Stream != nil {
		return runEndpointServiceStream(ctx, data, plan.EndpointCertificate, opened.HandshakeNonce, *plan.Stream)
	}
	return runEndpointService(ctx, data, plan.EndpointCertificate, opened.HandshakeNonce)
}

func writeControlFrame(connection interface{ Write([]byte) (int, error) }, kind frameType, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode native circuit control frame: %w", err)
	}
	return writeFrame(connection, frame{Type: kind, Payload: payload})
}

func randomCryptoHandle() (handle, error) {
	var value handle
	if _, err := rand.Read(value[:]); err != nil {
		return handle{}, fmt.Errorf("generate native circuit handle: %w", err)
	}
	return value, nil
}

func validateUserPlan(plan candidateUserPlan) error {
	if plan.Profile != candidateProfile && plan.Profile != c3Profile || plan.RunID == "" || plan.Rendezvous == "" || plan.Slot == (handle{}) || plan.HPKEPublic == nil || plan.EndpointTrust.Roots == nil || len(plan.Payload) > maximumQueueBytes {
		return errors.New("native C-5/C2 User plan is incomplete")
	}
	if plan.Stream != nil && validateStreamSpec(*plan.Stream, false) != nil || plan.Stream != nil && len(plan.Payload) != 0 ||
		plan.Attached != nil && (plan.Stream != nil || len(plan.Payload) != 0) {
		return errors.New("native C-5/C2 User stream plan is invalid")
	}
	dataLength, introductionLength := 3, 4
	if plan.Profile == c3Profile {
		dataLength, introductionLength = 2, 2
	}
	if len(plan.DataPath) != dataLength || len(plan.IntroductionPath) != introductionLength || plan.DataPath[dataLength-1].Address != plan.Rendezvous {
		return errors.New("native C-5/C2 User paths do not match the fixed topology")
	}
	return nil
}

func validateServicePlan(plan candidateServicePlan) error {
	if plan.Profile != candidateProfile && plan.Profile != c3Profile || plan.RunID == "" || plan.Rendezvous == "" || plan.Slot == (handle{}) || plan.HPKEPrivate == nil || len(plan.EndpointCertificate.Certificate) == 0 {
		return errors.New("native C-5/C2 Service plan is incomplete")
	}
	if plan.Stream != nil && validateStreamSpec(*plan.Stream, false) != nil || plan.Attached != nil && plan.Stream != nil {
		return errors.New("native C-5/C2 Service stream plan is invalid")
	}
	dataLength, introductionLength := 3, 3
	if plan.Profile == c3Profile {
		dataLength, introductionLength = 2, 2
	}
	if len(plan.DataPath) != dataLength || len(plan.IntroductionPath) != introductionLength || plan.DataPath[dataLength-1].Address != plan.Rendezvous {
		return errors.New("native C-5/C2 Service paths do not match the fixed topology")
	}
	return nil
}
