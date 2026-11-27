package connection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/recordio"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

// InitiatorBinding retains the independently verified destination and original
// Reader context. The local secret context/salt are not capsule fields.
type InitiatorBinding struct {
	self                          *InitiatorBinding
	operation                     *executionruntime.Operation
	setup                         context.Context
	identity                      wire.LogicalIdentity
	logical, attachment           [32]byte
	localContext                  [32]byte
	joinSecretDigest, joinContext [32]byte
	expires                       time.Time
	revision                      uint64
	recipientEnd                  time.Time
	proof                         publication.Proof
	source                        *InitiatorSource
}

// prepareInitiator fills only fresh logical fields under the already claimed
// original Source. It cannot claim a second operation or accept raw copied
// request facts as Source/JOIN authority.
func prepareInitiator(ctx context.Context, source *InitiatorSource, proof publication.Proof, profile [32]byte,
	request capsule.Request, localEnd time.Time) (*InitiatorBinding, capsule.Request, error) {
	if ctx == nil || source == nil || request.ConnectionNonce != [32]byte{} || request.InitiatorBinding != [32]byte{} {
		return nil, capsule.Request{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, capsule.Request{}, err
	}
	if err := source.check(); err != nil {
		return nil, capsule.Request{}, err
	}
	operation, localContext := source.config.Operation, source.localContext
	var salt [32]byte
	defer clear(salt[:])
	if _, err := rand.Read(salt[:]); err != nil || salt == [32]byte{} {
		return nil, capsule.Request{}, errors.New("initiator binding salt is unavailable")
	}
	if _, err := rand.Read(request.ConnectionNonce[:]); err != nil || request.ConnectionNonce == [32]byte{} {
		return nil, capsule.Request{}, errors.New("logical Connection nonce is unavailable")
	}
	transcript := append([]byte("ardents-initiator-binding-v3\x00"), salt[:]...)
	transcript = append(transcript, localContext[:]...)
	request.InitiatorBinding = sha256.Sum256(transcript)
	clear(transcript)
	digest, err := request.Digest()
	if err != nil {
		return nil, capsule.Request{}, err
	}
	derived, err := deriveRecipientGeneration(proof, profile, request, digest, localEnd, time.Now(), 1)
	if err != nil {
		return nil, capsule.Request{}, err
	}
	if end, exists := operation.Context().Deadline(); exists && time.Unix(request.WorkSafetyMaximum, 0).After(end) {
		return nil, capsule.Request{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, capsule.Request{}, err
	}
	if err := source.check(); err != nil {
		return nil, capsule.Request{}, err
	}
	binding := &InitiatorBinding{operation: operation, setup: ctx, identity: derived.Identity, logical: derived.Logical,
		attachment: derived.Attachment, localContext: localContext, expires: request.Deadline, revision: request.Revision, proof: proof,
		joinSecretDigest: sha256.Sum256(request.JoinSecret[:]), joinContext: request.HandshakeContext, source: source}
	binding.self = binding
	if err := binding.check(true); err != nil {
		return nil, capsule.Request{}, err
	}
	return binding, request, nil
}

func (binding *InitiatorBinding) check(setup bool) error {
	return binding.checkWithJoin(setup, nil)
}

func (binding *InitiatorBinding) checkWithJoin(setup bool, physical *join.Joined) error {
	if binding == nil || binding.self != binding || binding.operation == nil || binding.setup == nil || binding.source == nil {
		return errIntegrity
	}
	if err := binding.source.checkWithJoin(physical); err != nil {
		return err
	}
	localContext, err := binding.operation.ConnectionContext()
	if err != nil || localContext != binding.localContext {
		return errors.Join(errIntegrity, err)
	}
	now := time.Now()
	value := binding.proof.Delegation()
	if now.Before(value.NotBefore) || !now.Before(value.NotAfter) || !now.Before(time.Unix(binding.identity.WorkSafetyNotAfter, 0)) {
		return errIntegrity
	}
	if setup && (binding.setup.Err() != nil || !now.Before(binding.expires)) {
		return errors.Join(errIntegrity, binding.setup.Err())
	}
	if setup && !binding.recipientEnd.IsZero() && !now.Before(binding.recipientEnd) {
		return errIntegrity
	}
	return nil
}

// AuthenticateInitiator requires an actual retained JOIN and independently
// bound Reader destination. Exact certificate, protocol proof and coalesced
// zero-offset Continuity all precede accepted initial state.
func AuthenticateInitiator(ctx context.Context, binding *InitiatorBinding, physical *join.Joined) (_ *AuthenticatedAttachment, result error) {
	defer measurement.Begin(runtimeevent.ConnectionAuthenticate).End()

	if ctx == nil || binding == nil || binding.source == nil || physical == nil {
		return nil, errIntegrity
	}
	if err := errors.Join(ctx.Err(), binding.check(true), physical.CheckSourceBinding(binding.joinSecretDigest, binding.joinContext, time.Unix(binding.identity.WorkSafetyNotAfter, 0), binding.expires)); err != nil {
		return nil, err
	}
	logicalContext, cancelLogical, child, cancel := initiatorLifetimes(binding.operation.Context(), time.Unix(binding.identity.WorkSafetyNotAfter, 0))
	attachment := &initiatorAttachment{binding: binding, physical: physical, ctx: child, cancel: cancel, watcherDone: make(chan struct{})}
	attachment.self = attachment
	attachment.connection = newInstanceTLSClient(initiatorTLSBorrow{attachment: attachment}, binding.identity.Instance)
	owner := &AuthenticatedAttachment{role: wire.RoleInitiator, operation: binding.operation, initiator: attachment,
		identity: binding.identity, logical: binding.logical, attachment: binding.attachment,
		logicalContext: logicalContext, cancelLogical: cancelLogical, recipientRevision: binding.revision}
	owner.self = owner
	go attachment.watch()
	defer func() {
		if result != nil {
			result = errors.Join(result, owner.Close())
		}
	}()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); _ = owner.Close() })
	originalSetupDone := make(chan struct{})
	stopOriginalSetup := context.AfterFunc(binding.setup, func() { defer close(originalSetupDone); _ = owner.Close() })
	var callerOnce sync.Once
	joinCaller := func() {
		callerOnce.Do(func() {
			if !stopCaller() {
				<-callerDone
			}
			if !stopOriginalSetup() {
				<-originalSetupDone
			}
		})
	}
	defer joinCaller()
	if err := physical.SetDeadline(binding.expires); err != nil {
		return nil, err
	}
	if err := attachment.enter(); err != nil {
		return nil, err
	}
	err := attachment.connection.HandshakeContext(attachment.ctx)
	attachment.users.Done()
	if err := errors.Join(err, ctx.Err(), attachment.check()); err != nil {
		return nil, err
	}
	evidence, err := wire.BindTLS(attachment.connection, binding.attachment)
	if err != nil {
		return nil, err
	}
	attachment.mu.Lock()
	if err := attachment.checkLocked(); err != nil {
		attachment.mu.Unlock()
		evidence.Erase()
		return nil, err
	}
	attachment.evidence = evidence
	attachment.mu.Unlock()
	key, exporter := evidence.Secrets()
	defer clear(key[:])
	challenge := wire.Challenge{Network: binding.identity.Network, Target: binding.identity.Target, InstanceGeneration: binding.identity.InstanceGeneration, Context: binding.logical}
	if _, err := rand.Read(challenge.Nonce[:]); err != nil || challenge.Nonce == [32]byte{} {
		return nil, errIntegrity
	}
	request, err := wire.NewContinuity(key, wire.RoleInitiator, 1, 0, 0, 0, binding.logical, exporter)
	if err != nil || request.Nonce == challenge.Nonce {
		return nil, errors.Join(errIntegrity, err)
	}
	if err := recordio.WriteInitialRequest(attachment, challenge, request); err != nil {
		return nil, err
	}
	response := recordio.NewInitialResponseReader(attachment)
	first, err := readInitialResponseRecord(ctx, response)
	if err != nil {
		return nil, err
	}
	if first.Proof == nil {
		return nil, errIntegrity
	}
	second, err := readInitialResponseRecord(ctx, response)
	if err != nil {
		return nil, err
	}
	if second.Continuity == nil {
		return nil, errIntegrity
	}
	receipt, err := wire.VerifyPeerInitialResponse(evidence, binding.identity.Instance, challenge, request, *first.Proof, *second.Continuity)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), binding.check(true), attachment.check()); err != nil {
		return nil, err
	}
	if err := receipt.Consume(evidence, challenge); err != nil {
		return nil, err
	}
	if err := physical.SetDeadline(time.Unix(binding.identity.WorkSafetyNotAfter, 0)); err != nil {
		return nil, err
	}
	attachment.mu.Lock()
	if err := attachment.checkLocked(); err != nil {
		attachment.mu.Unlock()
		return nil, err
	}
	attachment.initialComplete = true
	attachment.mu.Unlock()
	joinCaller()
	if err := errors.Join(ctx.Err(), binding.check(true), owner.check()); err != nil {
		return nil, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return nil, errIntegrity
	}
	owner.key, owner.exporter = key, exporter
	return owner, nil
}

// newInstanceTLSClient binds the Reader's TLS peer to the exact verified Instance key.
// The Connection caller owns the original transport, handshake and proof lifetime.
func newInstanceTLSClient(conn net.Conn, expected [32]byte) *tls.Conn {
	return tls.Client(conn, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences:   transport.ClosedProfileKeyExchangeGroups(),
		InsecureSkipVerify: true, // The independently bound Instance is the verifier.
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyInstancePeer(state, expected)
		},
	})
}

// The setup caller can expire before its cancellation callback interrupts the
// physical Attachment. Retain that original cause after the read has joined;
// neither buffered complete bytes nor physical cancellation extend setup.
func readInitialResponseRecord(ctx context.Context, flight *recordio.InitialReader) (wire.Record, error) {
	if ctx == nil {
		return wire.Record{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return wire.Record{}, err
	}
	record, err := flight.ReadRecord()
	if errors.Is(err, wire.ErrInvalidRecord) {
		err = errors.Join(errIntegrity, err)
	}
	if err = errors.Join(err, ctx.Err()); err != nil {
		return wire.Record{}, err
	}
	return record, nil
}
