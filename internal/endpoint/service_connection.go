package endpoint

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func (endpoint *endpoint) connectAuthorized(ctx context.Context, input connectionInput, receipt broker.Receipt) (result runtimeResult, err error) {
	defer projectReceipt(&result, receipt)
	authority := endpoint.authority
	if input.AuthorityPublic != [32]byte{} {
		authority = input.AuthorityPublic
	}
	credential, err := decodePublication(input.Publication, authority, endpoint.network, input.At)
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
	_, err = authenticateInstance(attachment.connection, credential, connectionContext)
	if err != nil {
		return failed("service target authentication failure", "current Service Instance proof failed", err)
	}
	if input.OnAuthenticated != nil {
		if err := input.OnAuthenticated(credential.Target); err != nil {
			attachment.close()
			return failed("local authorization or policy denial", "local Reference presentation could not be opened", err)
		}
	}
	stream, streamErr := newNativeStream(ctx, input, credential, binding, nil, true, attachment, continuity,
		connectionContext, endpoint.resources)
	if streamErr != nil {
		attachment.close()
		return failed("service target authentication failure", "native Service Connection stream is invalid", streamErr)
	}
	sendBytes, receiveBytes := streamBounds(input)
	outcome, err := stream.RunBounded(sendBytes, receiveBytes)
	if err != nil {
		result, failure := streamFailure(ctx, outcome.Accepted, outcome.Received, err)
		result.AuthenticatedTarget, result.Generation = credential.Target, credential.Generation
		applyRecoveryOutcome(&result, outcome)
		return result, failure
	}
	return runtimeResult{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: outcome.Accepted,
		AcknowledgedBytes: outcome.Acknowledged, ReceivedBytes: outcome.Received,
		QueueHighWater:  outcome.QueueHigh,
		RouteGeneration: outcome.Generation, RecoveryCount: outcome.Recoveries,
		ContinuityCommitment: outcome.ContinuityCommitment}, nil
}

func (endpoint *endpoint) accept(ctx context.Context, input connectionInput) (result runtimeResult, err error) {
	session, activateErr := endpoint.activateApplicationSession(ctx, input.Session, input.Principal)
	if activateErr != nil {
		return denied(activateErr.Error())
	}
	defer session.Release()
	result, err = endpoint.acceptAuthorized(session.Context(), input, session.receipt)
	return preferCallerCancellation(ctx, result, err)
}

func (endpoint *endpoint) acceptAuthorized(ctx context.Context, input connectionInput, receipt broker.Receipt) (result runtimeResult, err error) {
	defer projectReceipt(&result, receipt)
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
	_, err = proveInstance(attachment.connection, credential, connectionContext, lease)
	if err != nil {
		return failed("service target authentication failure", "incoming exact Target proof failed", err)
	}
	stream, streamErr := newNativeStream(ctx, input, credential, binding, lease, false, attachment, continuity,
		connectionContext, endpoint.resources)
	if streamErr != nil {
		attachment.close()
		return failed("service target authentication failure", "native Service Connection stream is invalid", streamErr)
	}
	sendBytes, receiveBytes := streamBounds(input)
	outcome, err := stream.RunBounded(sendBytes, receiveBytes)
	if err != nil {
		result, failure := streamFailure(ctx, outcome.Accepted, outcome.Received, err)
		result.AuthenticatedTarget, result.Generation = credential.Target, credential.Generation
		applyRecoveryOutcome(&result, outcome)
		return result, failure
	}
	return runtimeResult{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: outcome.Accepted,
		AcknowledgedBytes: outcome.Acknowledged, ReceivedBytes: outcome.Received,
		QueueHighWater:  outcome.QueueHigh,
		RouteGeneration: outcome.Generation, RecoveryCount: outcome.Recoveries,
		ContinuityCommitment: outcome.ContinuityCommitment}, nil
}

func applyRecoveryOutcome(result *runtimeResult, outcome nativeconnection.Outcome) {
	result.AcknowledgedBytes = outcome.Acknowledged
	result.QueueHighWater = outcome.QueueHigh
	result.RouteGeneration = outcome.Generation
	result.RecoveryCount = outcome.Recoveries
	result.ContinuityCommitment = outcome.ContinuityCommitment
}

func newNativeStream(ctx context.Context, input connectionInput, credential publicationCredential, recovery routeRecovery, private crypto.Signer, client bool, initial *securedAttachment, continuity, connectionContext [32]byte,
	resources func(string, int) uint32) (*nativeconnection.Stream, error) {
	first, err := nativeAttachment(initial)
	if err != nil {
		return nil, err
	}
	var opener nativeconnection.AttachmentOpener
	if input.OpenAttachment != nil {
		opener = func(attempt context.Context, binding nativeconnection.Recovery) (*nativeconnection.Attachment, error) {
			raw, err := input.OpenAttachment(attempt, binding)
			if err != nil {
				return nil, err
			}
			var replacement *securedAttachment
			var fresh [32]byte
			if client {
				replacement, fresh, err = secureClient(attempt, raw, credential, connectionContext, binding.Generation)
			} else {
				replacement, fresh, err = securePublisher(attempt, raw, credential, private, connectionContext, binding.Generation)
			}
			erase(fresh[:])
			if errors.Is(err, errInstanceMismatch) {
				return nil, nativeconnection.ErrActiveViolation
			}
			if err != nil {
				return nil, err
			}
			return nativeAttachment(replacement)
		}
	}
	nameBinding, nameUpdates := input.NameBinding, input.NameUpdates
	if !client {
		nameBinding, nameUpdates = destinationBinding{}, nil
	}
	return nativeconnection.NewStream(nativeconnection.StreamConfig{Context: ctx, Application: input.Application,
		NetworkID: credential.NetworkID, Recovery: recovery, OpenAttachment: opener, Initial: first,
		ContinuityKey: continuity, Authorized: input.At, Client: client, NameBinding: nameBinding,
		NameUpdates: nameUpdates, CloseApplicationOnRemoteTerminal: input.closeApplicationOnRemoteTerminal, Resources: resources})
}

func nativeAttachment(attachment *securedAttachment) (*nativeconnection.Attachment, error) {
	return nativeconnection.NewAttachment(attachment.connection, attachment.generation, attachment.context,
		attachment.exporterCommitment, attachment.close)
}

func validateStreams(input connectionInput) error {
	sendBytes, receiveBytes := streamBounds(input)
	if input.Route == nil || input.Application == nil || sendBytes > maximumStreamBytes || receiveBytes > maximumStreamBytes ||
		(sendBytes == 0 && receiveBytes == 0) {
		return errors.New("stream or byte bound is missing")
	}
	return nil
}

func streamBounds(input connectionInput) (uint32, uint32) {
	if input.SendBytes == 0 && input.ReceiveBytes == 0 {
		return input.BytesEachDirection, input.BytesEachDirection
	}
	return input.SendBytes, input.ReceiveBytes
}

func validateRecoveryBinding(input connectionInput, credential publicationCredential) error {
	return nativeconnection.ValidateRecovery(input.OpenAttachment != nil, input.RecoveryBinding,
		input.At.Unix(), credential.NotAfter)
}

func authenticateInstance(connection io.ReadWriter, credential publicationCredential, connectionContext [32]byte) ([32]byte, error) {
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

func proveInstance(connection io.ReadWriter, credential publicationCredential, connectionContext [32]byte, signer crypto.Signer) ([32]byte, error) {
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
		return canary, errors.New("instance signer cannot prove this connection")
	}
	var proof [64]byte
	copy(proof[:], signature)
	return canary, nativeconnection.Write(connection, nativeconnection.Record{Proof: &nativeconnection.Proof{ChallengeDigest: digest, Signature: proof}})
}
