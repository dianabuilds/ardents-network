package serviceconn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

const challengeSize = 4 + 1 + 32 + 8 + 32
const proofSize = 4 + 1 + ed25519.SignatureSize

func (endpoint *endpoint) connect(ctx context.Context, input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "connection"); err != nil {
		return denied(err.Error())
	}
	credential, err := decodePublication(input.Publication, endpoint.authority, endpoint.network, input.At)
	if err != nil || input.Target == [32]byte{} || input.Target != credential.Target {
		return failed("service target authentication failure", "exact Service Target could not be authenticated", errors.New("target or publication mismatch"))
	}
	if err := validateNameOrigin(input, credential); err != nil {
		return failed("service target authentication failure", "Service Name binding is invalid", err)
	}
	if err := validateStreams(input); err != nil {
		return failed("local authorization or policy denial", "bounded local stream input is invalid", err)
	}
	if err := validateRecoveryBinding(input, credential); err != nil {
		return failed("local authorization or policy denial", "recovery binding is invalid", err)
	}
	binding := input.RecoveryBinding
	releaseConnection := acquireResource(endpoint.resources, "service-connection")
	defer releaseConnection()
	defer input.Route.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	attachment, continuity, err := secureClient(ctx, input.Route, credential, binding, 1)
	if err != nil {
		return failed("service target authentication failure", "current Service Instance TLS proof failed", err)
	}
	canary, err := authenticateInstance(attachment.connection, credential)
	if err != nil {
		return failed("service target authentication failure", "current Service Instance proof failed", err)
	}
	stream := newRecoveryStream(ctx, input.Application, credential, binding,
		nil, true, input.OpenAttachment, attachment, continuity, input.At,
		input.NameBinding, input.NameUpdates, endpoint.resources)
	sendBytes, receiveBytes := streamBounds(input)
	outcome, err := stream.run(sendBytes, receiveBytes)
	if err != nil {
		result, failure := streamFailure(ctx, outcome.accepted, outcome.received, err)
		applyRecoveryOutcome(&result, outcome)
		return result, failure
	}
	return Result{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: sendBytes,
		AcknowledgedBytes: outcome.acknowledged, ReceivedBytes: receiveBytes,
		ConnectionCanary: canary, QueueHighWater: outcome.queueHigh,
		RouteGeneration: outcome.generation, RecoveryCount: outcome.recoveries,
		ContinuityCommitment: outcome.continuity}, nil
}

func (endpoint *endpoint) accept(ctx context.Context, input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "connection"); err != nil {
		return denied(err.Error())
	}
	if err := validateStreams(input); err != nil {
		return failed("local authorization or policy denial", "bounded local stream input is invalid", err)
	}
	releaseConnection := acquireResource(endpoint.resources, "service-connection")
	defer releaseConnection()
	endpoint.mu.Lock()
	if endpoint.current == nil || input.At.Unix() < endpoint.current.credential.NotBefore || input.At.Unix() >= endpoint.current.credential.NotAfter {
		endpoint.mu.Unlock()
		return failed("service unavailable", "no current published Service Instance is available", errors.New("publication is absent or expired"))
	}
	credential := endpoint.current.credential
	private := append(ed25519.PrivateKey(nil), endpoint.current.private...)
	endpoint.mu.Unlock()
	if err := validateRecoveryBinding(input, credential); err != nil {
		return failed("local authorization or policy denial", "recovery binding is invalid", err)
	}
	binding := input.RecoveryBinding
	defer erase(private)
	defer endpoint.retire(credential.Generation)
	defer input.Route.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	attachment, continuity, err := securePublisher(ctx, input.Route, credential, private, binding, 1)
	if err != nil {
		return failed("service target authentication failure", "incoming Service Instance TLS proof failed", err)
	}
	canary, err := proveInstance(attachment.connection, credential, private)
	if err != nil {
		return failed("service target authentication failure", "incoming exact Target proof failed", err)
	}
	stream := newRecoveryStream(ctx, input.Application, credential, binding,
		private, false, input.OpenAttachment, attachment, continuity, input.At,
		DestinationBinding{}, nil, endpoint.resources)
	sendBytes, receiveBytes := streamBounds(input)
	outcome, err := stream.run(sendBytes, receiveBytes)
	if err != nil {
		result, failure := streamFailure(ctx, outcome.accepted, outcome.received, err)
		applyRecoveryOutcome(&result, outcome)
		return result, failure
	}
	return Result{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: sendBytes,
		AcknowledgedBytes: outcome.acknowledged, ReceivedBytes: receiveBytes,
		ConnectionCanary: canary, QueueHighWater: outcome.queueHigh,
		RouteGeneration: outcome.generation, RecoveryCount: outcome.recoveries,
		ContinuityCommitment: outcome.continuity}, nil
}

func applyRecoveryOutcome(result *Result, outcome recoveryOutcome) {
	result.AcknowledgedBytes = outcome.acknowledged
	result.QueueHighWater = outcome.queueHigh
	result.RouteGeneration = outcome.generation
	result.RecoveryCount = outcome.recoveries
	result.ContinuityCommitment = outcome.continuity
}

func validateStreams(input Request) error {
	sendBytes, receiveBytes := streamBounds(input)
	if input.Route == nil || input.Application == nil || sendBytes > maximumStreamBytes || receiveBytes > maximumStreamBytes ||
		(sendBytes == 0 && receiveBytes == 0) {
		return errors.New("stream or byte bound is missing")
	}
	return nil
}

func streamBounds(input Request) (uint32, uint32) {
	if input.SendBytes == 0 && input.ReceiveBytes == 0 {
		return input.BytesEachDirection, input.BytesEachDirection
	}
	return input.SendBytes, input.ReceiveBytes
}

func validateRecoveryBinding(input Request, credential Credential) error {
	binding := input.RecoveryBinding
	if input.OpenAttachment == nil {
		if binding != (Recovery{}) {
			return errors.New("recovery binding exists without an attachment opener")
		}
		return nil
	}
	if binding.CandidateView == [32]byte{} || binding.IsolationContext == [32]byte{} ||
		binding.DestinationBinding == [32]byte{} || len(binding.RouteProfile) == 0 || len(binding.RouteProfile) > 63 ||
		binding.WorkSafetyNotAfter <= input.At.Unix() || binding.WorkSafetyMaximum < binding.WorkSafetyNotAfter ||
		binding.WorkSafetyMaximum > credential.NotAfter || binding.NoNewRecoveryAfter <= input.At.Unix() ||
		binding.NoNewRecoveryAfter > binding.WorkSafetyNotAfter {
		return errors.New("fixed recovery values or finite safety bounds are incomplete")
	}
	return nil
}

func authenticateInstance(connection io.ReadWriter, credential Credential) ([32]byte, error) {
	var canary [32]byte
	challenge := make([]byte, challengeSize)
	copy(challenge[:4], "ASCH")
	challenge[4] = 1
	copy(challenge[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(challenge[37:45], credential.Generation)
	if _, err := rand.Read(challenge[45:]); err != nil {
		return canary, err
	}
	copy(canary[:], challenge[45:])
	if err := writeAll(connection, challenge); err != nil {
		return canary, err
	}
	proof := make([]byte, proofSize)
	if _, err := io.ReadFull(connection, proof); err != nil {
		return canary, err
	}
	if string(proof[:4]) != "ASPR" || proof[4] != 1 ||
		!ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]), proofMessage(challenge), proof[5:]) {
		return canary, errors.New("instance proof is invalid")
	}
	return canary, nil
}

func proveInstance(connection io.ReadWriter, credential Credential, private ed25519.PrivateKey) ([32]byte, error) {
	var canary [32]byte
	challenge := make([]byte, challengeSize)
	if _, err := io.ReadFull(connection, challenge); err != nil {
		return canary, err
	}
	if string(challenge[:4]) != "ASCH" || challenge[4] != 1 ||
		!equal32(challenge[5:37], credential.Target) || binary.BigEndian.Uint64(challenge[37:45]) != credential.Generation {
		return canary, errors.New("connection challenge does not bind current Target and generation")
	}
	copy(canary[:], challenge[45:])
	proof := make([]byte, proofSize)
	copy(proof[:4], "ASPR")
	proof[4] = 1
	copy(proof[5:], ed25519.Sign(private, proofMessage(challenge)))
	return canary, writeAll(connection, proof)
}

func proofMessage(challenge []byte) []byte {
	message := make([]byte, 0, 30+len(challenge))
	message = append(message, "ardents-h3-instance-proof-v1\x00"...)
	message = append(message, challenge...)
	return message
}

func equal32(value []byte, expected [32]byte) bool {
	if len(value) != len(expected) {
		return false
	}
	for index := range expected {
		if value[index] != expected[index] {
			return false
		}
	}
	return true
}
