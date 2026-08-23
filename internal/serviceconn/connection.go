package serviceconn

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

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
	publicationDigest := sha256.Sum256(input.Publication)
	connectionContext, err := connectionContext(credential, binding, publicationDigest)
	if err != nil {
		return failed("service target authentication failure", "native Service Connection context is invalid", err)
	}
	releaseConnection := acquireResource(endpoint.resources, "service-connection")
	defer releaseConnection()
	defer input.Route.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	attachment, continuity, err := secureClient(ctx, input.Route, credential, connectionContext, 1)
	if err != nil {
		return failed("service target authentication failure", "current Service Instance TLS proof failed", err)
	}
	canary, err := authenticateInstance(attachment.connection, credential, connectionContext)
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
	if endpoint.publications == nil {
		return failed("service unavailable", "no current published Service Instance is available", errors.New("publication is absent or expired"))
	}
	lease, acquireErr := endpoint.publications.AcquireAt(ctx, input.At)
	if acquireErr != nil {
		return failed("service unavailable", "no current published Service Instance is available", acquireErr)
	}
	defer lease.Close()
	credential := lease.Current().Credential
	if err := validateRecoveryBinding(input, credential); err != nil {
		return failed("local authorization or policy denial", "recovery binding is invalid", err)
	}
	binding := input.RecoveryBinding
	connectionContext, contextErr := connectionContext(credential, binding, lease.Current().Digest)
	if contextErr != nil {
		return failed("service target authentication failure", "native Service Connection context is invalid", contextErr)
	}
	defer input.Route.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	attachment, continuity, err := securePublisher(ctx, input.Route, credential, lease, connectionContext, 1)
	if err != nil {
		return failed("service target authentication failure", "incoming Service Instance TLS proof failed", err)
	}
	canary, err := proveInstance(attachment.connection, credential, connectionContext, lease)
	if err != nil {
		return failed("service target authentication failure", "incoming exact Target proof failed", err)
	}
	stream := newRecoveryStream(ctx, input.Application, credential, binding,
		lease, false, input.OpenAttachment, attachment, continuity, input.At,
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

func authenticateInstance(connection io.ReadWriter, credential Credential, connectionContext [32]byte) ([32]byte, error) {
	var canary [32]byte
	if _, err := rand.Read(canary[:]); err != nil {
		return canary, err
	}
	challenge := nativeconnection.Challenge{Network: credential.NetworkID, Target: credential.Target,
		InstanceGeneration: credential.Generation, Context: connectionContext, Nonce: canary}
	if err := nativeconnection.Write(connection, nativeconnection.Record{Challenge: &challenge}); err != nil {
		return canary, err
	}
	record, err := nativeconnection.Read(connection)
	if err != nil {
		return canary, err
	}
	digest, err := nativeconnection.ChallengeDigest(challenge)
	if err != nil || record.Proof == nil || record.Proof.ChallengeDigest != digest ||
		!ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]), digest[:], record.Proof.Signature[:]) {
		return canary, errors.New("instance proof is invalid")
	}
	return canary, nil
}

func proveInstance(connection io.ReadWriter, credential Credential, connectionContext [32]byte, signer crypto.Signer) ([32]byte, error) {
	var canary [32]byte
	record, err := nativeconnection.Read(connection)
	if err != nil {
		return canary, err
	}
	challenge := record.Challenge
	if challenge == nil || challenge.Network != credential.NetworkID || challenge.Target != credential.Target ||
		challenge.InstanceGeneration != credential.Generation || challenge.Context != connectionContext {
		return canary, errors.New("connection challenge does not bind current Target and generation")
	}
	canary = challenge.Nonce
	digest, err := nativeconnection.ChallengeDigest(*challenge)
	if err != nil {
		return canary, err
	}
	signature, err := signer.Sign(nil, digest[:], crypto.Hash(0))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return canary, errors.New("Instance signer cannot prove this connection")
	}
	var proof [64]byte
	copy(proof[:], signature)
	return canary, nativeconnection.Write(connection, nativeconnection.Record{Proof: &nativeconnection.Proof{ChallengeDigest: digest, Signature: proof}})
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
