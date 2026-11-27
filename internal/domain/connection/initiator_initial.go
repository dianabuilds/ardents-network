package connection

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
	"github.com/dianabuilds/ardents-network/internal/domain/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
)

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
	if source.config.PrepareRoute != nil {
		if err := source.config.PrepareRoute(source.ctx, handle, intro, rendezvous); err != nil {
			return nil, fmt.Errorf("connection initial Route preparation: %w", err)
		}
	}
	setupEnd := attachmentSetupDeadline(source.ctx, time.Now(), source.config.WorkSafetyNotAfter, descriptor.Introduction.NotAfter)
	if !time.Now().Before(setupEnd) {
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
		if err := introduction.SubmitScoped(ctx, handle, intro, envelope, source.config.Exclusions); err != nil {
			return fmt.Errorf("connection initial Introduction submission: %w", err)
		}
		return nil
	}, func() error {
		var err error
		physical, err = acquisition.Join(source.physical, join.Config{Duty: rendezvous, Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: source.config.WorkSafetyNotAfter, SetupDeadline: setupEnd})
		if err != nil {
			return fmt.Errorf("connection initial JOIN: %w", err)
		}
		return nil
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
