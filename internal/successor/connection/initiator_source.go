package connection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// InitiatorSourceConfig names original owners, not a replacement policy. On
// successful construction Source transfers to Connection; History and the
// retained Rendezvous selection remain caller-owned through Source.Close.
type InitiatorSourceConfig struct {
	Operation                                                 *executionruntime.Operation
	Source                                                    *prefix.Prefix
	PrepareSource                                             func(context.Context) (prefix.Config, error)
	History                                                   *reachability.History
	Rendezvous                                                *selection.Rendezvous
	TargetLink                                                string
	Network, Profile                                          [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter time.Time
	Exclusions                                                []route.Member
}

// InitiatorSource retains one original Reader Source through lookup, capsule
// submission, JOIN and exact Instance authentication. An initial result is not
// a recovery source or permission to expose an ordinary Application stream.
type InitiatorSource struct {
	self              *InitiatorSource
	config            InitiatorSourceConfig
	scope             *prefix.SourceScope
	target            [32]byte
	localContext      [32]byte
	ctx               context.Context
	cancel            context.CancelFunc
	physical          context.Context
	physicalCancel    context.CancelFunc
	mu                sync.Mutex
	closed, attempted bool
	flights           sync.WaitGroup
	owner             *AuthenticatedAttachment
	stream            *Stream
	failure           error
	once              sync.Once
	closeErr          error
}

// NewInitiatorSource checks original authority and finite bounds before taking
// ownership of the actual Source. Failure leaves that physical owner with its
// caller without clearing independent history. The sole logical construction
// claim precedes ownership transfer and every lookup/presentation effect;
// failed construction never closes a caller-owned or sibling Source.
func NewInitiatorSource(config InitiatorSourceConfig) (*InitiatorSource, error) {
	if config.Operation == nil || config.Source == nil || config.History == nil || config.Rendezvous == nil {
		return nil, errIntegrity
	}
	networkID, target, err := publication.ParseTargetLink(config.TargetLink)
	if err != nil || networkID != config.Network || config.Profile == [32]byte{} {
		return nil, errors.Join(errIntegrity, err)
	}
	for _, bound := range []time.Time{config.WorkSafetyNotAfter, config.WorkSafetyMaximum, config.NoNewRecoveryAfter} {
		if bound.IsZero() || bound != bound.UTC().Truncate(time.Second) || !time.Now().Before(bound) {
			return nil, errIntegrity
		}
	}
	if config.WorkSafetyNotAfter.After(config.WorkSafetyMaximum) || config.NoNewRecoveryAfter.After(config.WorkSafetyNotAfter) {
		return nil, errIntegrity
	}
	if err := config.Operation.CheckReader(); err != nil {
		return nil, err
	}
	if end, exists := config.Operation.Context().Deadline(); exists && config.WorkSafetyMaximum.After(end) {
		return nil, errIntegrity
	}
	if err := config.Source.CheckSourceCaller(config.Operation.Context(), config.Network, config.Profile, config.WorkSafetyMaximum); err != nil {
		return nil, err
	}
	localContext, err := config.Operation.ConnectionContext()
	if err != nil {
		return nil, err
	}
	config.Exclusions = append([]route.Member(nil), config.Exclusions...)
	physical, physicalCancel := context.WithDeadline(config.Operation.Context(), config.WorkSafetyMaximum)
	ctx, cancel := context.WithCancel(physical)
	source := &InitiatorSource{config: config, target: target, localContext: localContext, ctx: ctx, cancel: cancel, physical: physical, physicalCancel: physicalCancel}
	source.self = source
	if err := source.check(); err != nil {
		cancel()
		physicalCancel()
		return nil, err
	}
	// Claim under the original Source seal barrier. No physical observation,
	// opening or join runs inside this local handoff.
	err = config.Source.CommitPair(nil, func(checkOriginal func() error) error {
		if err := errors.Join(checkOriginal(), ctx.Err(), config.Operation.CheckReader()); err != nil {
			return err
		}
		return config.Operation.ClaimConnection(localContext)
	})
	if err == nil {
		err = source.check()
	}
	if err != nil {
		cancel()
		physicalCancel()
		return nil, err
	}
	source.scope, err = prefix.NewSourceScope(config.Operation.Context(), config.Source, config.PrepareSource)
	if err != nil {
		cancel()
		physicalCancel()
		return nil, err
	}
	return source, nil
}

// localLocked runs no Source observation or physical I/O. The caller holds mu.
func (source *InitiatorSource) localLocked() error {
	if source.failure != nil || source.ctx.Err() != nil {
		return errors.Join(source.failure, source.ctx.Err())
	}
	if source.closed {
		return net.ErrClosed
	}
	actual, err := source.config.Operation.ConnectionContext()
	if err != nil {
		return err
	}
	if actual != source.localContext || actual == [32]byte{} {
		return errIntegrity
	}
	return nil
}

func (source *InitiatorSource) check() error {
	if source == nil || source.self != source || source.ctx == nil || source.config.Operation == nil || source.config.Source == nil {
		return errIntegrity
	}
	source.mu.Lock()
	err := source.localLocked()
	source.mu.Unlock()
	if err != nil {
		return err
	}
	var current error
	if source.scope == nil {
		// Constructor checks the actual original Prefix before its scope
		// transfer. No constructed consumer uses this preparation path.
		current = source.config.Source.CheckSourceCaller(source.config.Operation.Context(), source.config.Network, source.config.Profile, source.config.WorkSafetyMaximum)
	} else {
		current = source.scope.CheckSourceCaller(source.config.Operation.Context(), source.config.Network, source.config.Profile, source.config.WorkSafetyMaximum)
	}
	if current != nil {
		return current
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.localLocked()
}

// AuthenticateInitial consumes one setup attempt over the SAME original
// Source acquisition. Lookup history survives rejected TLS and worker loss.
func (source *InitiatorSource) AuthenticateInitial() (_ *AuthenticatedAttachment, result error) {
	if err := source.check(); err != nil {
		return nil, err
	}
	source.mu.Lock()
	if err := source.localLocked(); err != nil || source.attempted {
		source.mu.Unlock()
		return nil, errors.Join(errIntegrity, err)
	}
	source.attempted = true
	source.flights.Add(1)
	source.mu.Unlock()
	defer source.flights.Done()
	defer func() {
		if result != nil {
			source.mu.Lock()
			if source.failure == nil {
				source.failure = result
			}
			source.cancel()
			source.mu.Unlock()
		}
	}()
	handle, err := source.scope.Acquire(source.ctx)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	acquisition, err := join.AcquireScopedSourceJoin(source.physical, handle)
	if err != nil {
		return nil, err
	}
	transferred := false
	defer func() {
		if !transferred {
			result = errors.Join(result, acquisition.Close())
		}
	}()
	// Local setup cancellation interrupts this acquisition only while JOIN is
	// opening. TLS later owns its users and joins them before physical return.
	stopped := make(chan struct{})
	stop := context.AfterFunc(source.ctx, func() { defer close(stopped); _ = acquisition.Close() })
	var stopOnce sync.Once
	joinSetupCancellation := func() {
		stopOnce.Do(func() {
			if !stop() {
				<-stopped
			}
		})
	}
	defer joinSetupCancellation()
	lookupEnd := time.Now().Add(30 * time.Second).UTC().Truncate(time.Second)
	if lookupEnd.After(source.config.WorkSafetyNotAfter) {
		lookupEnd = source.config.WorkSafetyNotAfter
	}
	candidate, err := handle.LookupDescriptor(source.ctx, lookupEnd, source.target, source.config.History, source.config.Exclusions)
	if err != nil {
		return nil, err
	}
	// Exported DTO fields are mutable. Only independently verified signed bytes
	// may supply the recipient and Publication facts used below.
	descriptor, err := reachability.Verify(candidate.Bytes(), source.target, source.config.Network, source.config.Profile, time.Now())
	if err != nil {
		return nil, err
	}
	if err := source.check(); err != nil {
		return nil, err
	}
	intro, err := handle.ResolveSubmission(descriptor.Introduction.Node, source.config.Exclusions)
	if err != nil {
		return nil, err
	}
	excluded := append(append([]route.Member(nil), source.config.Exclusions...), route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	rendezvous, until, err := handle.Rendezvous(source.config.Rendezvous, 0, excluded)
	if err != nil || source.config.WorkSafetyMaximum.After(until) {
		return nil, errors.Join(errIntegrity, err)
	}
	if err := handle.CheckRendezvous(rendezvous, excluded); err != nil {
		return nil, err
	}
	setupEnd := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	if setupEnd.After(source.config.WorkSafetyNotAfter) {
		setupEnd = source.config.WorkSafetyNotAfter
	}
	if setupEnd.After(descriptor.Introduction.NotAfter) {
		return nil, errIntegrity
	}
	setup, cancel := context.WithDeadline(source.ctx, setupEnd)
	defer cancel()
	request := capsule.Request{Network: source.config.Network, Target: source.target, PublicationDigest: descriptor.PublicationDigest,
		Revision: descriptor.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: source.config.Profile, AttachmentGeneration: 1, Deadline: setupEnd,
		WorkSafetyNotAfter: source.config.WorkSafetyNotAfter.Unix(), WorkSafetyMaximum: source.config.WorkSafetyMaximum.Unix(), NoNewRecoveryAfter: source.config.NoNewRecoveryAfter.Unix()}
	defer clear(request.JoinSecret[:])
	if _, err := rand.Read(request.JoinSecret[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(request.HandshakeContext[:]); err != nil {
		return nil, err
	}
	binding, filled, err := prepareInitiator(setup, source, descriptor.Publication, source.config.Profile, request, source.config.WorkSafetyMaximum)
	if err != nil {
		return nil, err
	}
	defer clear(filled.JoinSecret[:])
	request = filled
	header := capsule.Header{Slot: descriptor.Introduction.Slot, Revision: request.Revision, Expiry: setupEnd}
	if _, err := rand.Read(header.DeliveryNonce[:]); err != nil {
		return nil, err
	}
	envelope, _, err := capsule.Seal(header, descriptor.Introduction.RecipientKey, request)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(source.check(), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, err
	}
	var physical *join.Joined
	err = runInitiatorSetupFlights(setup, acquisition.Close, func(ctx context.Context) error {
		return introduction.SubmitScoped(ctx, handle, intro, envelope, source.config.Exclusions)
	}, func() error {
		var err error
		physical, err = acquisition.Join(source.physical, join.JoinConfig{Duty: rendezvous, Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: source.config.WorkSafetyNotAfter, SetupDeadline: setupEnd})
		return err
	})
	if err != nil {
		return nil, err
	}
	joinSetupCancellation()
	if err := errors.Join(source.check(), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, err
	}
	owner, err := AuthenticateInitiator(setup, binding, physical)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !transferred {
			result = errors.Join(result, owner.Close())
		}
	}()
	if err := errors.Join(source.check(), owner.check(), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, err
	}
	err = handle.Commit(func(checkOriginal func() error) error {
		source.mu.Lock()
		defer source.mu.Unlock()
		if err := errors.Join(checkOriginal(), source.localLocked(), descriptor.Current(time.Now()), setup.Err()); err != nil {
			return err
		}
		source.owner = owner
		transferred = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return owner, nil
}

// OpenStream installs this genuine original replacement source before record
// actors start. It consumes only the same original initial authentication;
// replacement never constructs another Application operation or byte contract.
func (source *InitiatorSource) OpenStream(contract executionruntime.StreamContract) (_ *Stream, result error) {
	if err := source.check(); err != nil {
		return nil, err
	}
	source.mu.Lock()
	if err := source.localLocked(); err != nil || source.stream != nil {
		source.mu.Unlock()
		return nil, errors.Join(errIntegrity, err)
	}
	source.flights.Add(1)
	owner := source.owner
	source.mu.Unlock()
	defer source.flights.Done()
	if owner == nil {
		var err error
		owner, err = source.AuthenticateInitial()
		if err != nil {
			return nil, err
		}
	}
	stream, err := owner.openStream(contract, nil, source)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, stream.Close())
			source.config.Operation.RetainCleanup(stream.JoinedCleanup())
		}
	}()
	if err := source.check(); err != nil {
		return nil, err
	}
	err = source.scope.Commit(func(checkOriginal func() error) error {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		source.mu.Lock()
		defer source.mu.Unlock()
		if source.stream != stream || source.owner != owner || stream.closing || stream.ctx.Err() != nil {
			return errIntegrity
		}
		return errors.Join(checkOriginal(), source.localLocked(), source.config.Operation.CheckReader())
	})
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// openReplacement uses the SAME original Source acquisition for this finite
// proposal's current lookup, recipient, fresh capsule and actual JOIN. The
// caller reserved its ticket before any acquisition or irreversible spend.
func (source *InitiatorSource) openReplacement(proposal *initiatorReplacement) (*InitiatorBinding, *join.Joined, error) {
	if err := proposal.check(); err != nil {
		return nil, nil, err
	}
	handle, err := source.scope.Acquire(proposal.ctx)
	if err != nil {
		return nil, nil, errors.Join(err, source.scope.RetiredWork())
	}
	proposal.sourceHandle = &initiatorRouteAcquisition{handle: handle}
	acquisition, err := join.AcquireScopedSourceJoin(proposal.stream.owner.context(), handle)
	if err != nil {
		return nil, nil, err
	}
	proposal.acquisition = acquisition
	proposal.stopInterruption = context.AfterFunc(proposal.ctx, func() {
		defer close(proposal.interruptionDone)
		proposal.interruptErr = acquisition.Close()
	})
	lookupEnd := proposal.ticket.deadline.UTC().Truncate(time.Second)
	candidate, err := handle.LookupDescriptor(proposal.ctx, lookupEnd, source.target, source.config.History, source.config.Exclusions)
	if err != nil {
		return nil, nil, fmt.Errorf("connection replacement Descriptor lookup: %w", err)
	}
	descriptor, err := reachability.Verify(candidate.Bytes(), source.target, source.config.Network, source.config.Profile, time.Now())
	if err != nil {
		return nil, nil, err
	}
	stream := proposal.stream
	stream.mu.Lock()
	revision := stream.recipientRevision
	stream.mu.Unlock()
	value := descriptor.Publication.Delegation()
	identity := stream.owner.identity
	if descriptor.PublicationDigest != identity.Publication || value.Instance != identity.Instance || value.Generation != identity.InstanceGeneration || descriptor.Introduction.Revision < revision {
		return nil, nil, proposal.failIntegrity(nil)
	}
	if err := errors.Join(proposal.check(), descriptor.Current(time.Now())); err != nil {
		return nil, nil, err
	}
	intro, err := handle.ResolveSubmission(descriptor.Introduction.Node, source.config.Exclusions)
	if err != nil {
		return nil, nil, err
	}
	excluded := append(append([]route.Member(nil), source.config.Exclusions...), route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	// At most the existing alternate is used; failed attempts cannot redraw or
	// expand the original context's retained Rendezvous set.
	rendezvous, until, err := handle.Rendezvous(source.config.Rendezvous, proposal.slot, excluded)
	if err != nil || source.config.WorkSafetyMaximum.After(until) {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	if err := handle.CheckRendezvous(rendezvous, excluded); err != nil {
		return nil, nil, err
	}
	setupEnd := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	if lookupEnd.Before(setupEnd) {
		setupEnd = lookupEnd
	}
	if descriptor.Introduction.NotAfter.Before(setupEnd) || !time.Now().Before(setupEnd) {
		return nil, nil, errRecoveryExhausted
	}
	request := capsule.Request{Network: identity.Network, Target: identity.Target, PublicationDigest: identity.Publication,
		Revision: descriptor.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: identity.Profile, AttachmentGeneration: proposal.ticket.generation, Deadline: setupEnd,
		ConnectionNonce: identity.ConnectionNonce, InitiatorBinding: identity.InitiatorBinding,
		WorkSafetyNotAfter: identity.WorkSafetyNotAfter, WorkSafetyMaximum: identity.WorkSafetyMaximum, NoNewRecoveryAfter: identity.NoNewRecoveryAfter}
	defer clear(request.JoinSecret[:])
	if _, err := rand.Read(request.JoinSecret[:]); err != nil {
		return nil, nil, err
	}
	if _, err := rand.Read(request.HandshakeContext[:]); err != nil {
		return nil, nil, err
	}
	digest, err := request.Digest()
	if err != nil {
		return nil, nil, err
	}
	derived, err := deriveRecipientGeneration(descriptor.Publication, identity.Profile, request, digest, source.config.WorkSafetyMaximum, time.Now(), proposal.ticket.generation)
	if err != nil || derived.identity != identity || derived.logical != stream.owner.logical {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	binding := &InitiatorBinding{operation: source.config.Operation, setup: proposal.ctx, identity: derived.identity, logical: derived.logical,
		attachment: derived.attachment, localContext: source.localContext, expires: setupEnd, revision: request.Revision,
		recipientEnd: descriptor.Introduction.NotAfter, proof: descriptor.Publication, source: source,
		joinSecretDigest: sha256.Sum256(request.JoinSecret[:]), joinContext: request.HandshakeContext}
	binding.self = binding
	header := capsule.Header{Slot: descriptor.Introduction.Slot, Revision: request.Revision, Expiry: setupEnd}
	if _, err := rand.Read(header.DeliveryNonce[:]); err != nil {
		return nil, nil, err
	}
	envelope, _, err := capsule.Seal(header, descriptor.Introduction.RecipientKey, request)
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true)); err != nil {
		return nil, nil, err
	}
	setup, cancel := context.WithDeadline(proposal.ctx, setupEnd)
	defer cancel()
	var physical *join.Joined
	err = runInitiatorSetupFlights(setup, acquisition.Close, func(ctx context.Context) error {
		if err := introduction.SubmitScoped(ctx, handle, intro, envelope, source.config.Exclusions); err != nil {
			return fmt.Errorf("connection replacement Introduction submission: %w", err)
		}
		return nil
	}, func() error {
		var err error
		physical, err = acquisition.Join(stream.owner.context(), join.JoinConfig{Duty: rendezvous,
			Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: source.config.WorkSafetyNotAfter, SetupDeadline: setupEnd})
		if err != nil {
			return fmt.Errorf("connection replacement JOIN: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, nil, err
	}
	return binding, physical, nil
}

// runInitiatorSetupFlights joins the two independent setup exchanges. The
// callers above alone construct genuine Submission/JOIN operations and retain
// their authority checks and durable presentations before operation bytes.
// JOIN keeps its original physical caller: canceling this setup must interrupt
// the exact acquisition, not shorten a successfully transferred stream.
func runInitiatorSetupFlights(setup context.Context, interrupt func() error, submit func(context.Context) error, openJoin func() error) error {
	ctx, cancel := context.WithCancel(setup)
	defer cancel()
	interrupted := make(chan struct{})
	var interruption error
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		interruption = interrupt()
	})
	submitted := make(chan error, 1)
	go func() {
		err := submit(ctx)
		if err != nil {
			cancel()
		}
		submitted <- err
	}()
	joined := openJoin()
	if joined != nil {
		cancel()
	}
	submission := <-submitted
	// Disarm and join before canceling our private setup context on return.
	// Any already-started physical retirement and its error remain retained.
	if !stop() {
		<-interrupted
	}
	return errors.Join(submission, joined, interruption, ctx.Err())
}

// The exact physical handle stays within this permitted Route import owner.
type initiatorRouteAcquisition struct{ handle *prefix.SourceHandle }

func (route *initiatorRouteAcquisition) Commit(action func(func() error) error) error {
	if route == nil || route.handle == nil {
		return errIntegrity
	}
	return route.handle.Commit(action)
}

func (route *initiatorRouteAcquisition) close() {
	if route != nil {
		route.handle.Close()
	}
}

// Close seals attempts before joining TLS and any rejected opening, then the
// original Source. It never clears history, closes shared selection or renews
// local authority. All physically joined cleanup errors remain retained.
func (source *InitiatorSource) Close() error {
	if source == nil {
		return nil
	}
	if source.self != source {
		return errIntegrity
	}
	source.once.Do(func() {
		source.mu.Lock()
		source.closed = true
		source.cancel()
		owner := source.owner
		stream := source.stream
		source.mu.Unlock()
		var work, physical error
		if stream != nil {
			work = stream.Close()
			if work != nil {
				work = fmt.Errorf("connection original Reader stream retirement: %w", work)
			}
			physical = stream.JoinedCleanup()
		}
		if owner != nil {
			// The initial Attachment may already have failed and been replaced.
			// Its own Close retains that historical work result. It cannot
			// turn an authenticated recovered logical result into old EOF.
			_ = owner.Close()
			physical = errors.Join(physical, owner.cleanupErr)
		}
		source.flights.Wait()
		if err := source.scope.Close(); err != nil {
			work = errors.Join(work, fmt.Errorf("connection original Source scope retirement: %w", err))
		}
		cleanup, complete := source.scope.CleanupResult()
		if !complete {
			cleanup = errors.Join(cleanup, errIntegrity)
		}
		physical = errors.Join(physical, cleanup)
		source.physicalCancel()
		source.config.Operation.RetainCleanup(physical)
		source.mu.Lock()
		source.closeErr = errors.Join(source.failure, work, physical)
		source.mu.Unlock()
	})
	return source.closeErr
}
