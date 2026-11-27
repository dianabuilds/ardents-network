package connection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/domain/route/selection"
)

// InitiatorSourceConfig names original owners, not a replacement policy. On
// successful construction Source transfers to Connection; History and the
// retained Rendezvous selection remain caller-owned through Source.Close.
type InitiatorSourceConfig struct {
	Operation     *executionruntime.Operation
	Source        *prefix.Prefix
	PrepareSource func(context.Context) (prefix.Config, error)
	// PrepareRoute funds the already selected Introduction and Rendezvous
	// through this exact retained Source before the short capsule setup starts.
	// Nil means Stock is provisioned independently; actual HELLO presentation
	// and all subsequent authority checks remain required in either case.
	PrepareRoute                                              func(context.Context, *prefix.SourceHandle, network.RetainedDuty, network.RetainedDuty) error
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
	return source.checkWithJoin(nil)
}

func (source *InitiatorSource) checkWithJoin(physical *join.Joined) error {
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
	if physical != nil {
		if source.scope == nil {
			return errIntegrity
		}
		current = physical.CheckSourceCaller(source.scope, source.config.Operation.Context(), source.config.Network, source.config.Profile, source.config.WorkSafetyMaximum)
	} else if source.scope == nil {
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

// attachmentSetupDeadline selects the original absolute setup bound once.
// Rounding cannot extend a caller's earlier deadline (ADR-0121).
func attachmentSetupDeadline(ctx context.Context, now time.Time, bounds ...time.Time) time.Time {
	end := now.Add(30 * time.Second)
	if callerEnd, ok := ctx.Deadline(); ok && callerEnd.Before(end) {
		end = callerEnd
	}
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end.UTC().Truncate(time.Second)
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
