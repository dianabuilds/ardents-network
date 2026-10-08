package installation

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"syscall"
	"time"
)

// This lifetime belongs to one attempted fixed-unit start, not to a copied PID
// or journal receipt. Failure retains physical custody until actual quiescence;
// a full ACK is an irreversible handoff to independently observed startup.
type installedCandidate struct {
	preparation    *successorPreparation
	process        *installedProcessPin
	scopes         *cgroup.Lifetime
	startAttempted bool
	stopRequested  bool
	joined         bool
	released       bool
	ackWritten     bool
	terminal       error
}

func (owned *successorPreparation) startCandidate() (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.candidate == nil || owned.stage == nil ||
		owned.stage.barrier == nil || owned.stage.barrier.owner != owned || owned.started != nil || owned.terminal != nil {
		return ErrBinding
	}
	if _, bounded := owned.ctx.Deadline(); !bounded {
		return ErrInput
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	if err := owned.stage.barrier.observe(); err != nil {
		return err
	}
	// Preserve the attempted effect before contacting the manager. A visible
	// intention is not a successful Start; uncertainty stays repair-required.
	if err := owned.stage.barrier.recordStartAttempt(); err != nil {
		return err
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	if err := owned.stage.barrier.observe(); err != nil {
		return err
	}
	original := &installedCandidate{preparation: owned}
	owned.started = original
	defer func() {
		returnedErr = errors.Join(returnedErr, owned.ctx.Err())
		if returnedErr != nil {
			original.terminal = errors.Join(original.terminal, returnedErr)
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			// Once all ACK bytes may be visible, a later error must not describe
			// this invocation as never admitted or issue a second ACK.
			if !original.acknowledged() {
				original.terminal = errors.Join(original.terminal, original.quiesce())
				owned.terminal = errors.Join(owned.terminal, original.terminal)
				returnedErr = errors.Join(returnedErr, original.terminal)
			}
			owned.stage.retainFailure(owned.ctx, returnedErr)
			owned.terminal = errors.Join(owned.terminal, owned.stage.terminal)
			returnedErr = errors.Join(returnedErr, owned.stage.terminal)
		}
	}()
	// Even an error may follow a real manager effect. Own the attempt BEFORE it.
	original.startAttempted = true
	if err := systemd.Start(owned.ctx); err != nil {
		return err
	}
	if err := original.retain(owned.ctx); err != nil {
		return err
	}
	if err := original.scopes.Observe(); err != nil {
		return err
	}
	completionErr := owned.completeCandidateStart(original.process)
	original.ackWritten = original.acknowledged()
	if !original.ackWritten {
		return completionErr
	}
	// Full output cannot be revoked by a late error. Cleanup remains bound to
	// this archived invocation and must not enter the never-admitted stop path.
	return errors.Join(completionErr, owned.stage.barrier.cleanupArchivedStart(owned.stage, original.process))
}

func (original *installedCandidate) acknowledged() bool {
	return original != nil && (original.ackWritten || original.preparation != nil && original.preparation.stage != nil &&
		original.preparation.stage.barrier != nil && original.preparation.stage.barrier.owner == original.preparation &&
		original.process != nil && original.preparation.stage.barrier.accepted == original.process &&
		original.preparation.stage.barrier.ackWritten)
}

type startedInvocationObservation struct {
	Schema           string `json:"schema"`
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
	PID              uint32 `json:"pid"`
	Invocation       string `json:"invocation_id"`
}

// Detached journal bytes describe an observation, not replayable authority.
func observedCandidateStart(pin *installedProcessPin) ([]byte, error) {
	if pin == nil || pin.pid == 0 || pin.invocation == [16]byte{} ||
		pin.previous.selected.Schema != "ardents-endpoint-installation-selection-v1" ||
		!canonicalDigest(pin.previous.selected.GenerationDigest) || !canonicalDigest(pin.previous.selected.BindingDigest) {
		return nil, ErrBinding
	}
	return canonicalJSON(startedInvocationObservation{
		Schema:           "ardents-endpoint-installation-start-observation-v1",
		GenerationDigest: pin.previous.selected.GenerationDigest, BindingDigest: pin.previous.selected.BindingDigest,
		PID: pin.pid, Invocation: hex.EncodeToString(pin.invocation[:]),
	})
}

func (owned *successorPreparation) observeAcceptedCandidate(pin *installedProcessPin) error {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil ||
		owned.predecessor == nil || !owned.predecessor.joined || pin == nil || pin.reader != owned.inspection ||
		owned.candidate == nil || pin.previous.selected != owned.candidate.selected || pin.previous.selected != owned.stage.selected || owned.stage.barrier == nil ||
		owned.stage.barrier.owner != owned || owned.stage.barrier.accepted != pin ||
		owned.started == nil || owned.started.process != pin || owned.started.scopes == nil || owned.started.stopRequested {
		return ErrBinding
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.stage.barrier.observeCompletionInvocation(pin); err != nil {
		return err
	}
	if err := owned.started.scopes.Observe(); err != nil {
		return err
	}
	return owned.observe()
}

// Private producer: the original start lifetime retains these exact process
// and scope observations. An accepting new runtime consumer is still required.
// Records never replace actual native admission.
func (owned *successorPreparation) completeCandidateStart(pin *installedProcessPin) (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil ||
		owned.predecessor == nil || !owned.predecessor.joined || pin == nil || pin.reader != owned.inspection ||
		owned.candidate == nil || pin.previous.selected != owned.candidate.selected || pin.previous.selected != owned.stage.selected || owned.stage.barrier == nil ||
		owned.stage.barrier.owner != owned || owned.stage.archivedIntent || owned.started == nil ||
		owned.started.process != pin || owned.started.scopes == nil || owned.started.stopRequested {
		return ErrBinding
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
		}
	}()
	if err := owned.observe(); err != nil {
		return err
	}
	stage := owned.stage
	if err := stage.verifyTransitionPhase("0007.json", "successor-reloaded-stopped"); err != nil {
		return err
	}
	if err := stage.barrier.acceptCompletion(pin); err != nil {
		return err
	}
	if err := owned.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	started, err := observedCandidateStart(pin)
	if err != nil {
		return err
	}
	if err := stage.journal.Write(owned.ctx, journal.Transitions, "started-invocation.json", started); err != nil {
		return err
	}
	if err := owned.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	if err := stage.copyAndRemoveIntent(owned.ctx); err != nil {
		return err
	}
	if err := owned.observeAcceptedCandidate(pin); err != nil {
		return err
	}
	return stage.barrier.acknowledgeArchivedStart(stage, pin)
}

func (original *installedCandidate) retain(ctx context.Context) error {
	if original == nil || original.preparation == nil || original.preparation.candidate == nil ||
		ctx == nil || !original.startAttempted || original.stopRequested || original.process != nil {
		return ErrBinding
	}
	owned := original.preparation
	checked := *owned.candidate
	if err := owned.inspection.observe(ctx); err != nil {
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
	pid, invocation, err := endpointManagerInvocation(unit, service)
	if err != nil {
		return err
	}
	if err := verifyRunningEndpointProperties(version, unit, service, checked.request, checked.selected.GenerationDigest, pid, invocation); err != nil {
		return err
	}
	original.process, err = pinOriginalProcess(owned.ctx, ctx, owned.inspection, checked, pid, invocation)
	if err != nil {
		return err
	}
	// A partial native scope result is retained on error after this real start.
	if original.scopes == nil {
		original.scopes, err = cgroup.RetainStarted(ctx, pid, checked.binding.UID)
		if err != nil {
			return err
		}
	}
	if err := observeBoundActivationSockets(ctx); err != nil {
		return err
	}
	return original.observeInvocation(ctx)
}

func (original *installedCandidate) observeInvocation(ctx context.Context) error {
	if original == nil || original.preparation == nil || original.preparation.candidate == nil ||
		original.process == nil || original.scopes == nil || ctx == nil {
		return ErrBinding
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	unit, service, err := systemd.Endpoint(ctx)
	if err != nil {
		return err
	}
	checked, pin := original.preparation.candidate, original.process
	if err := verifyRunningEndpointProperties(version, unit, service, checked.request, checked.selected.GenerationDigest, pin.pid, pin.invocation); err != nil {
		return err
	}
	return errors.Join(pin.observeOriginalProcess(ctx), original.scopes.Observe(), ctx.Err())
}

func (original *installedCandidate) quiesce() error {
	if original == nil || original.preparation == nil || original.preparation.ctx == nil ||
		original.preparation.candidate == nil || !original.startAttempted || original.acknowledged() {
		return ErrBinding
	}
	if original.joined {
		return original.terminal
	}
	owned := original.preparation
	cleanup := context.WithoutCancel(owned.ctx)
	if original.scopes != nil {
		joined, err := original.scopes.Joined()
		if err != nil {
			original.terminal = errors.Join(original.terminal, err)
		}
		if err == nil && joined {
			// Natural exit may retire the retained proc directory before failure
			// cleanup. Do not require a live process or stop a new invocation:
			// original kernel join and fresh fixed-unit quiescence decide release.
			return original.joinAttempt(cleanup)
		}
	}
	if !original.stopRequested {
		if original.process == nil {
			version, err := systemd.Version(cleanup)
			if err != nil {
				return errors.Join(original.terminal, err)
			}
			unit, service, err := systemd.Endpoint(cleanup)
			if err != nil {
				return errors.Join(original.terminal, err)
			}
			checked := owned.candidate
			if err := errors.Join(owned.inspection.observe(cleanup), verifyEndpointConfiguration(version, unit, service, checked.request, checked.selected.GenerationDigest)); err != nil {
				return errors.Join(original.terminal, err)
			}
			if !systemd.Matches(service, "MainPID", "u", uint32(0)) {
				if err := original.retain(cleanup); err != nil {
					original.terminal = errors.Join(original.terminal, err)
					if original.process == nil || original.scopes == nil {
						return original.terminal
					}
				}
			} else {
				// Pending/failed start may have an original scope even without
				// MainPID. No worker is allowed before this unacknowledged handoff.
				if original.scopes == nil {
					var err error
					original.scopes, err = cgroup.RetainStarted(cleanup, 0, checked.binding.UID)
					if err != nil {
						if original.scopes == nil {
							if !errors.Is(err, syscall.ENOENT) {
								original.terminal = errors.Join(original.terminal, err)
								return original.terminal
							}
						} else {
							// Partial inventory refusal, including a lost worker,
							// cannot abandon the retained Endpoint descriptor.
							original.terminal = errors.Join(original.terminal, err)
						}
					}
				}
			}
		} else {
			if original.scopes == nil {
				var err error
				original.scopes, err = cgroup.RetainStarted(cleanup, original.process.pid, owned.candidate.binding.UID)
				if err != nil {
					original.terminal = errors.Join(original.terminal, err)
					if original.scopes == nil {
						return original.terminal
					}
				}
			}
			if err := original.observeInvocation(cleanup); err != nil {
				original.terminal = errors.Join(original.terminal, err)
				// A retired or changed process denies Stop authority. Still join
				// every original descriptor; neither refusal nor proc absence
				// can release the lease while original descendants remain.
				return original.joinAttempt(cleanup)
			}
		}
		original.stopRequested = true
		stopErr := systemd.Stop(cleanup)
		original.terminal = errors.Join(original.terminal, stopErr, owned.ctx.Err())
		if original.scopes == nil && stopErr != nil {
			// An unobserved queued start cannot be released after a failed stop.
			return original.terminal
		}
	}
	return original.joinAttempt(cleanup)
}

// Failed-attempt join has no Stop or admission authority. Retained original
// descriptors survive a missing process, cancellation and manager replacement;
// a fresh stopped/empty/no-Job observation is still required before release.
func (original *installedCandidate) joinAttempt(cleanup context.Context) error {
	if original == nil || cleanup == nil || original.preparation == nil || original.preparation.ctx == nil ||
		original.preparation.candidate == nil || original.preparation.inspection == nil || !original.startAttempted || original.acknowledged() {
		return ErrBinding
	}
	original.terminal = errors.Join(original.terminal, original.preparation.ctx.Err())
	if original.scopes == nil && !original.stopRequested {
		return ErrBinding
	}
	if original.scopes != nil {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var firstObservation error
		for {
			joined, err := original.scopes.Joined()
			if err != nil && firstObservation == nil {
				firstObservation = err
			}
			if err == nil && joined {
				original.terminal = errors.Join(original.terminal, firstObservation)
				break
			}
			<-ticker.C
		}
	}
	if err := original.observeQuiescent(cleanup); err != nil {
		return errors.Join(original.terminal, err)
	}
	original.joined = true
	return original.terminal
}

func (original *installedCandidate) observeQuiescent(ctx context.Context) error {
	owned := original.preparation
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	for range 2 {
		unit, service, err := systemd.Endpoint(ctx)
		if err != nil {
			return err
		}
		checked := owned.candidate
		if err := verifyStoppedAttemptProperties(version, unit, service, checked.request, checked.selected.GenerationDigest); err != nil {
			return err
		}
		// A stopped observation with a queued job is not a finished attempt.
		if err := verifyNoQueuedEndpointJob(unit); err != nil {
			return err
		}
		if err := errors.Join(observeStoppedActivationSockets(ctx), observeStoppedActivationManager(ctx), cgroup.ObserveEmpty(ctx), owned.inspection.observe(ctx)); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (original *installedCandidate) close() error {
	if original == nil {
		return nil
	}
	if original.released {
		return original.terminal
	}
	if !original.acknowledged() {
		if !original.joined {
			original.terminal = errors.Join(original.terminal, original.quiesce())
		}
		if !original.joined {
			return errors.Join(original.terminal, ErrBinding)
		}
	}
	original.terminal = errors.Join(original.terminal, original.scopes.Close(), original.process.close())
	original.scopes, original.process = nil, nil
	original.released = true
	return original.terminal
}
