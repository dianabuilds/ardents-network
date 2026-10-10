package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// This retained observation has requested no stop. A transition must retain
// these exact pins through subsequent stop and physical join, not replace them
// with observations of a later invocation or renewed caller.
type installedPredecessor struct {
	preparation      *successorPreparation
	process          *installedProcessPin
	pid              uint32
	invocation       [16]byte
	scopes           *cgroup.Lifetime
	managerReference *systemd.EndpointReference
	stopRequested    bool
	joinRequired     bool
	joined           bool
	terminal         error
	alreadyStopped   bool
	loadedRequest    installationRequest
	loadedGeneration string
}

func (owned *successorPreparation) retainPredecessor() (original *installedPredecessor, returnedErr error) {
	if owned == nil || owned.predecessor != nil {
		return nil, ErrInput
	}
	if err := owned.observe(); err != nil {
		return nil, err
	}
	reference, err := systemd.RetainEndpoint(owned.ctx)
	defer func() {
		if reference != nil {
			returnedErr = errors.Join(returnedErr, reference.Close())
		}
	}()
	if err != nil {
		return nil, err
	}
	version, err := systemd.Version(owned.ctx)
	if err != nil {
		return nil, err
	}
	unit, service, err := systemd.Endpoint(owned.ctx)
	if err != nil {
		return nil, err
	}
	if systemd.Matches(unit, "ActiveState", "s", "inactive") && systemd.Matches(unit, "SubState", "s", "dead") &&
		systemd.Matches(service, "MainPID", "u", uint32(0)) {
		request, generation, err := owned.matchQuiescentConfiguration(version, unit, service)
		if err != nil {
			return nil, err
		}
		retained := &installedPredecessor{preparation: owned, managerReference: reference, alreadyStopped: true, joined: true, loadedRequest: request, loadedGeneration: generation}
		reference = nil // transferred before any subsequent observation can fail
		// The marker becomes owned only after repeated native stopped properties,
		// loaded-worker inventory and actual empty kernel scopes all agree.
		if err := retained.observe(); err != nil {
			return nil, errors.Join(err, retained.close())
		}
		owned.predecessor = retained
		return retained, nil
	}
	// A recorded interrupted fixed write already required predecessor join.
	// Recovery may only observe continuing quiescence, never stop a live
	// invocation whose configuration is now partially replaced.
	if owned.recovery != nil && owned.recovery.fixedMutation {
		return nil, ErrBinding
	}
	pid, invocation, err := endpointManagerInvocation(unit, service)
	if err != nil {
		return nil, err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, owned.previous.request, owned.previous.selected.GenerationDigest, pid, invocation); err != nil {
		return nil, err
	}
	if err := observeBoundActivationSockets(owned.ctx); err != nil {
		return nil, err
	}
	retained := &installedPredecessor{preparation: owned, managerReference: reference, pid: pid, invocation: invocation, loadedRequest: owned.previous.request, loadedGeneration: owned.previous.selected.GenerationDigest}
	reference = nil
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, retained.close())
			original = nil
		}
	}()
	retained.process, err = pinInstalledProcess(owned.ctx, owned.inspection, owned.previous, pid, invocation)
	if err != nil {
		return nil, err
	}
	retained.scopes, err = cgroup.Retain(owned.ctx, pid, owned.previous.binding.UID)
	if err != nil {
		return nil, err
	}
	// Kernel pinning is followed by a fresh manager observation of the same
	// invocation and another original proc-directory/byte/credential check.
	if err := retained.observe(); err != nil {
		return nil, err
	}
	owned.predecessor = retained
	return retained, nil
}

func (original *installedPredecessor) observe() error {
	if original != nil && original.alreadyStopped {
		if original.preparation == nil || !original.joined || original.stopRequested || original.joinRequired {
			return ErrInput
		}
		owned := original.preparation
		if err := owned.observe(); err != nil {
			return err
		}
		if err := original.observeQuiescent(owned.ctx); err != nil {
			return err
		}
		return owned.observe()
	}
	if original == nil || original.preparation == nil || original.process == nil || original.scopes == nil || original.stopRequested || original.joinRequired {
		return ErrInput
	}
	owned := original.preparation
	if err := owned.observe(); err != nil {
		return err
	}
	if err := original.observeOriginalInvocation(owned.ctx); err != nil {
		return err
	}
	return owned.observe()
}

// This identity check is also used by failure-only cleanup. It grants no new
// native admission and cannot renew the cancelled preparation's authority.
func (original *installedPredecessor) observeOriginalInvocation(ctx context.Context) error {
	if original == nil || original.preparation == nil || original.process == nil || original.scopes == nil || ctx == nil {
		return ErrInput
	}
	owned := original.preparation
	if err := original.managerReference.Observe(ctx); err != nil {
		return err
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	unit, service, err := systemd.Endpoint(ctx)
	if err != nil {
		return err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, owned.previous.request, owned.previous.selected.GenerationDigest, original.pid, original.invocation); err != nil {
		return err
	}
	if err := original.process.observeOriginalProcess(ctx); err != nil {
		return err
	}
	if err := observeBoundActivationSockets(ctx); err != nil {
		return err
	}
	if err := original.scopes.Observe(); err != nil {
		return err
	}
	unit, service, err = systemd.Endpoint(ctx)
	if err != nil {
		return err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, owned.previous.request, owned.previous.selected.GenerationDigest, original.pid, original.invocation); err != nil {
		return err
	}
	if err := original.process.observeOriginalProcess(ctx); err != nil {
		return err
	}
	return ctx.Err()
}

// Both previously stopped and physically joined live predecessors require a
// current quiescent manager and real kernel inventory before fixed writes.
func (original *installedPredecessor) observeQuiescent(ctx context.Context) error {
	if original == nil || original.preparation == nil || !original.joined || ctx == nil || (!original.alreadyStopped && !original.stopRequested) {
		return ErrInput
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	for range 2 {
		if err := original.managerReference.Observe(ctx); err != nil {
			return err
		}
		unit, service, err := systemd.Endpoint(ctx)
		if err != nil {
			return err
		}
		if err := verifyQuiescentEndpointProperties(version, unit, service, original.loadedRequest, original.loadedGeneration); err != nil {
			return err
		}
		if err := observeStoppedActivationSockets(ctx); err != nil {
			return err
		}
		if err := cgroup.ObserveEmpty(ctx); err != nil {
			return err
		}
		if err := observeStoppedManager(ctx); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// Pre-stop refusal can close observations. After a stop request, its original
// physical join must be established before releasing any descriptor custody.
func (original *installedPredecessor) close() error {
	if original == nil {
		return nil
	}
	if (original.stopRequested || original.joinRequired) && !original.joined {
		return ErrBinding
	}
	var returnedErr error
	returnedErr = errors.Join(returnedErr, original.scopes.Close())
	original.scopes = nil
	returnedErr = errors.Join(returnedErr, original.process.close())
	original.process = nil
	returnedErr = errors.Join(returnedErr, original.managerReference.Close())
	original.managerReference = nil
	return returnedErr
}

// Cancellation denies transition admission, but cannot complete a requested
// stop or free the original physical pins. Fixed writes follow verified join.
func (original *installedPredecessor) stopAndJoin() error {
	if original != nil && original.alreadyStopped {
		if err := original.observe(); err != nil {
			return err
		}
		if original.preparation.stage != nil {
			return original.preparation.replaceFixedResources()
		}
		return nil
	}
	if original == nil || original.preparation == nil || original.process == nil || original.scopes == nil {
		return ErrInput
	}
	if original.stopRequested {
		if !original.joined {
			return errors.Join(ErrBinding, original.terminal, original.preparation.ctx.Err())
		}
		return errors.Join(original.terminal, original.preparation.ctx.Err())
	}
	if err := original.observe(); err != nil {
		return err
	}
	original.stopRequested = true
	original.joinRequired = true
	// This context serves physical quiescence only. It cannot admit a successor,
	// renew Release authority, clear a failure or replace the original caller.
	cleanup := context.WithoutCancel(original.preparation.ctx)
	stopErr := systemd.Stop(cleanup)
	original.terminal = errors.Join(stopErr, original.preparation.ctx.Err())
	joinErr := original.joinOriginal()
	original.terminal = errors.Join(original.terminal, joinErr,
		original.observeQuiescent(cleanup), original.preparation.ctx.Err())
	if original.terminal == nil && original.preparation.stage != nil {
		original.terminal = original.preparation.replaceFixedResources()
	}
	return original.terminal
}

// A failed intent attempt owns physical quiescence even when its original
// caller has cancelled. Refuse to stop an observed replacement invocation, but
// keep the original pins and writer lease until their actual kernel join.
func (original *installedPredecessor) quiesceAfterFailure() error {
	if original == nil || original.preparation == nil || original.preparation.ctx == nil || original.process == nil || original.scopes == nil {
		return ErrInput
	}
	if original.joined {
		return errors.Join(original.terminal, original.preparation.ctx.Err())
	}
	original.joinRequired = true
	cleanup := context.WithoutCancel(original.preparation.ctx)
	if !original.stopRequested {
		if err := original.observeOriginalInvocation(cleanup); err != nil {
			original.terminal = errors.Join(original.terminal, err)
		} else {
			original.stopRequested = true
			err := systemd.Stop(cleanup)
			original.terminal = errors.Join(original.terminal, err)
		}
	}
	original.terminal = errors.Join(original.terminal, original.preparation.ctx.Err(), original.joinOriginal(),
		observeStoppedManager(cleanup), cgroup.ObserveEmpty(cleanup), original.preparation.ctx.Err())
	return original.terminal
}

// Retain the first failed kernel observation and continue owning every original
// pin. A wait timeout or caller cancellation is never physical completion.
func (original *installedPredecessor) joinOriginal() error {
	if original == nil || original.scopes == nil || (!original.stopRequested && !original.joinRequired) {
		return ErrInput
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var firstObservation error
	for {
		joined, err := original.scopes.Joined()
		if err != nil && firstObservation == nil {
			firstObservation = err
		}
		if err == nil && joined {
			original.joined = true
			return firstObservation
		}
		<-ticker.C
	}
}

// Phase constrains expected configuration, never establishes physical quiescence.
// The caller independently repeats actual manager, activation and kernel checks.
func (owned *successorPreparation) matchQuiescentConfiguration(version string, unit, service managerProperties) (installationRequest, string, error) {
	if owned == nil {
		return installationRequest{}, "", ErrInput
	}
	// Fresh manager retention can load candidate unit bytes after the original
	// reference dies, before selection or reload completes. Exact fixed completion
	// already requires all nine original records and complete candidate images.
	// This matches loaded facts only; fresh proofs still precede every effect.
	if owned.recovery != nil && (owned.recovery.inventory["0004.json"] || owned.recovery.inventory["0006.json"]) {
		candidate := owned.recovery.checked
		err := verifyQuiescentEndpointProperties(version, unit, service, candidate.request, candidate.selected.GenerationDigest)
		if err == nil {
			return candidate.request, candidate.selected.GenerationDigest, nil
		}
		if owned.recovery.inventory["0007.json"] {
			return installationRequest{}, "", err
		}
	}
	if err := verifyQuiescentEndpointProperties(version, unit, service, owned.previous.request, owned.previous.selected.GenerationDigest); err != nil {
		return installationRequest{}, "", err
	}
	return owned.previous.request, owned.previous.selected.GenerationDigest, nil
}
