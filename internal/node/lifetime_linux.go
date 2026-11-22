//go:build linux

package node

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// nodeLifetime is the one original role-lifetime mechanism shared by Forwarding,
// Issuer, Descriptor, Introduction and JOIN: the admission/denial latch, the
// independent supervisor, physical retirement and retained completion. Each role
// embeds it and supplies its genuine per-role behavior through nodeRole. The
// shared mechanism owns no role policy: acquisition ordering, the full effect
// observation, the failed-observation notice policy and the role-owner cleanup
// remain with each role and must not be homogenized.
type nodeLifetime struct {
	failure                  failureRecord
	mu                       sync.Mutex
	caller, ctx              context.Context
	cancel                   context.CancelFunc
	authority                role.Authority
	roleName                 string
	participationRoot        string
	spendRoot, hostingRoot   string
	denied                   bool
	first, result            error
	cleanupResult            error
	retired                  error
	firstOwner, cleanupOwner string
	done                     chan struct{}
	process                  *processresource.Guard
	producer                 *duty.NodeProducer
	budget                   *hosting.Budget
	receiving                *receiving.Owner
	role                     *receiver.Receiver
	hooks                    nodeRole
}

// nodeRole carries the genuine per-role behavior the shared skeleton calls back.
// current is the full role effect observation; failedObservation is the role's
// notice policy (bare cause for Forwarding/Issuer, retired notice for the other
// roles); denyOwner notifies the role owner of denial; cleanupRole releases the
// role owner after Route join; supervise is the role's independent watchdog.
type nodeRole interface {
	current() (network.RuntimeView, error)
	admit(context.Context, receiver.Channel, []byte) (receiving.Grant, error)
	failedObservation(error, string) error
	denyOwner(cause error)
	cleanupRole(roleResult error) (error, func(func(string, error)))
	credentialClose() error
	certificate() tls.Certificate
	listenerConfig() receiver.ReceiverConfig
	supervise()
}

// newNodeLifetime constructs the shared state after the role has validated its
// configuration and caller. The retirement notice carries the role name; the
// authority is the original command-owned observer and is never rebound here.
func newNodeLifetime(ctx context.Context, authority role.Authority, roleName, participationRoot, spendRoot, hostingRoot string) nodeLifetime {
	child, cancel := context.WithCancel(ctx)
	return nodeLifetime{
		failure:           failureRecord{started: time.Now()},
		caller:            ctx,
		ctx:               child,
		cancel:            cancel,
		authority:         authority,
		roleName:          roleName,
		participationRoot: participationRoot,
		spendRoot:         spendRoot,
		hostingRoot:       hostingRoot,
		retired:           errors.New("original " + roleName + " admission retired"),
		done:              make(chan struct{}),
	}
}

// admission rechecks the original caller and the denial latch without holding
// the state lock across foreign observation. A derived context's propagation may
// lag, so the original caller itself is checked first.
func (l *nodeLifetime) admission() error {
	l.mu.Lock()
	denied := l.denied
	l.mu.Unlock()
	if err := l.caller.Err(); err != nil {
		l.deny(err, "caller")
		return l.retired
	}
	if denied || l.ctx.Err() != nil {
		return l.retired
	}
	return nil
}

// currentBase observes the original native process, participation producer and
// signed State without borrowing any role owner. Each observation is followed by
// direct caller checks. Roles that additionally check a role owner compose this
// base observation; Forwarding and JOIN keep their own full observations.
func (l *nodeLifetime) currentBase() (network.RuntimeView, error) {
	if err := l.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	measurement, err := l.process.Check()
	if err != nil {
		return network.RuntimeView{}, l.hooks.failedObservation(err, "process")
	}
	if err := l.producer.Check(); err != nil {
		return network.RuntimeView{}, l.hooks.failedObservation(err, "participation")
	}
	view, err := l.authority.Current()
	owner := "network"
	if err == nil && view.ProfileBinding() != l.authority.Profile {
		err = errors.New("original " + l.roleName + " profile changed")
	}
	if err == nil {
		err = view.MatchDuty(l.authority.Duty, time.Now())
	}
	if err == nil && time.Since(measurement.Completed) > time.Second {
		err = expiredMeasurement(measurement)
		owner = "process"
	}
	if err != nil {
		return network.RuntimeView{}, l.hooks.failedObservation(err, owner)
	}
	if err := l.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	return view, nil
}

// retiredObservation is the notice policy shared by Introduction, Descriptor and
// JOIN: dependents receive the original retirement notice, never a foreign
// operational cause mislabeled as their own physical Close failure. Node retains
// that cause. A bare caller/cancellation error is lent the retirement notice.
func (l *nodeLifetime) retiredObservation(err error, owner string) error {
	if err == l.retired || l.caller.Err() != nil && err == l.caller.Err() || l.ctx.Err() != nil && err == l.ctx.Err() {
		l.deny(l.caller.Err(), "caller")
		return l.retired
	}
	l.deny(err, owner)
	return l.retired
}

// deny is the denial latch shared by Issuer, Descriptor, Introduction and JOIN.
// Forwarding keeps its own latch (bare-cause notice policy and runtime owner
// default). The role owner is notified only after the state lock is released.
func (l *nodeLifetime) deny(cause error, owner string) {
	l.mu.Lock()
	l.denied = true
	if l.first == nil && cause != nil && cause != l.retired {
		l.first, l.firstOwner = cause, owner
		l.failure.retain(cause, l.caller, owner)
	}
	l.mu.Unlock()
	l.hooks.denyOwner(cause)
	l.cancel()
}

// stop synchronously denies new effects from the original caller.
func (l *nodeLifetime) stop() {
	l.deny(l.caller.Err(), "caller")
}

// observeReceiving projects one coherent receiving observation for the roles
// whose receiving notice carries the role name. Forwarding keeps its own
// observeAdmission with the original generic notice.
func (l *nodeLifetime) observeReceiving(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := l.hooks.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving "+l.roleName+" duty changed"), err)
	}
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: admissionFacts(p), Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

// supervise is the independent Node watchdog shared by Issuer, Descriptor,
// Introduction and JOIN. Route's own watcher already calls current every 100 ms;
// this supervisor rechecks the full role observation on the same cadence and
// joins physical cleanup on denial, route completion or a failed observation.
// Forwarding keeps its own supervisor over the local-only check.
func (l *nodeLifetime) supervise() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			l.deny(l.caller.Err(), "caller")
		case <-l.role.Done():
			l.deny(nil, "route")
		case <-ticker.C:
			if _, err := l.hooks.current(); err == nil {
				continue
			}
		}
		l.finish()
		return
	}
}

// finish joins physical cleanup and publishes the retained terminal result.
func (l *nodeLifetime) finish() {
	result := l.cleanup()
	l.mu.Lock()
	l.failure.retainCleanup(result, l.caller, l.cleanupOwner)
	l.cleanupResult = result
	l.result = errors.Join(l.first, result)
	l.mu.Unlock()
	close(l.done)
}

// cleanup joins every acquired physical user in the original order: Route, the
// role owner, receiving, Hosting, participation producer, process and credential.
// Receiver.Close returns only after physical join, even when its retained
// physical result fails. No bounded waiting context releases these owners. The
// role owner is released through cleanupRole, which preserves each role's exact
// attribution and release filtering after the Route result is retained.
func (l *nodeLifetime) cleanup() error {
	var result error
	retain := func(owner string, cause error) {
		if cause != nil && l.cleanupOwner == "" {
			l.cleanupOwner = owner
		}
		result = errors.Join(result, cause)
	}
	var roleResult error
	if l.role != nil {
		roleResult = originalRetirementResult(l.role.Close(), l.retired)
	}
	routeResult, afterRoute := l.hooks.cleanupRole(roleResult)
	retain("route", routeResult)
	if afterRoute != nil {
		afterRoute(retain)
	}
	if l.receiving != nil {
		retain("admission", l.receiving.Close())
	}
	if l.budget != nil {
		retain("hosting", l.budget.Close())
	}
	if l.producer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		retain("participation", l.producer.Close(ctx))
		cancel()
	}
	retain("process", l.process.Close())
	retain("credential", l.hooks.credentialClose())
	return result
}

// joinedStartupFailure wraps a rejected partial startup after the role has denied
// it: the caller receives the original cause joined with the filtered retirement
// notice and the physically joined cleanup, never the private notice alone.
func (l *nodeLifetime) joinedStartupFailure(result error) error {
	l.mu.Lock()
	first := l.first
	l.mu.Unlock()
	result = errors.Join(first, originalRetirementResult(result, l.retired), l.cleanup())
	return &startupFailure{cause: result, fact: l.failure.snapshot(l.first)}
}

// serve acquires the listener effects after every required owner and original
// participation precede them. A later listener/startup failure joins any acquired
// physical users before cleanup removes this conservative live producer. The
// authority lent to Route rebinds Current to the role observation on a local copy;
// the base observation keeps the original command-owned observer.
func (l *nodeLifetime) serve() error {
	if err := l.producer.Live(); err != nil {
		return err
	}
	authority := l.authority
	authority.Current = l.hooks.current
	config := l.hooks.listenerConfig()
	config.Authority = authority
	config.Certificate = l.hooks.certificate()
	config.Admit = l.hooks.admit
	var err error
	l.role, err = receiver.Listen(l.ctx, config)
	if err != nil {
		return err
	}
	if _, err := l.hooks.current(); err != nil {
		return err
	}
	l.mu.Lock()
	l.failure.serving = true
	l.mu.Unlock()
	go l.hooks.supervise()
	return nil
}

// openProducer opens the original participation producer with the role's root and
// the original member binding. Retained openings never create missing history.
func (l *nodeLifetime) openProducer(retained bool, m network.Member) error {
	var err error
	l.producer, err = duty.OpenNodeProducer(l.ctx, duty.Config{Root: l.participationRoot, Clock: time.Now, Create: !retained}, duty.NodeParticipation{Identity: m.NodeID, FamilyID: m.FamilyID, Assignment: m.Assignment, EpochUntil: l.authority.Duty.Epoch.ValidUntil, RecordUntil: m.NotAfter()})
	return err
}

// openHosting opens the original Hosting budget root.
func (l *nodeLifetime) openHosting() error {
	var err error
	l.budget, err = hosting.Open(l.hostingRoot)
	return err
}

// openReceiving opens the original receiving owner, retained or fresh, with the
// role's observation callback.
func (l *nodeLifetime) openReceiving(retained bool, binding receiving.Receiver, observe func() (receiving.Observation, error)) error {
	open := receiving.Open
	if retained {
		open = receiving.OpenRetained
	}
	var err error
	l.receiving, err = open(l.spendRoot, binding, observe)
	return err
}

// receivingBinding projects the original profile/member binding shared by every
// role's receiving owner.
func receivingBinding(view network.RuntimeView, m network.Member) receiving.Receiver {
	p := view.ProfileBinding()
	return receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
}

// acceptWithReservation is the receiving admission body shared by Introduction,
// Descriptor and JOIN: reobserve the full role effect, reserve actual work and
// termination, cover the joint class allowance, reobserve again, then hold the
// reservation through the Route borrower. Each role keeps its own purpose check.
func (l *nodeLifetime) acceptWithReservation(ctx context.Context, class admission.Class, raw []byte, c receiver.Channel, work, termination hosting.Traffic) (receiving.Grant, error) {
	return l.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		if _, err := l.hooks.current(); err != nil {
			return nil, err
		}
		held, err := l.budget.Reserve(ctx, hosting.ReservationRequest{Work: work, Termination: termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
		if err != nil {
			return nil, err
		}
		limit := class.ByteLimit()
		if err := l.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			return nil, errors.Join(err, releaseReservation(held))
		}
		if _, err := l.hooks.current(); err != nil {
			return nil, errors.Join(err, releaseReservation(held))
		}
		return c.HoldReservation(func() error { return releaseReservation(held) })
	})
}

// joinedOutcome is a bounded diagnostic after actual completion. It never grants
// admission and never classifies a physical cleanup error as cancellation.
func (l *nodeLifetime) joinedOutcome() string {
	<-l.done
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cleanupResult != nil || l.first != nil && l.first != context.Canceled {
		return "failed"
	}
	return "canceled"
}

// joinedFailureOwners projects fixed local owner categories after actual join.
func (l *nodeLifetime) joinedFailureOwners() (first, cleanup string) {
	<-l.done
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.firstOwner, l.cleanupOwner
}

// firstFailure returns detached original facts without observing or waiting for
// completion.
func (l *nodeLifetime) firstFailure() *FailureDiagnostic {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.failure.snapshot(l.first)
}

// closeResult waits for actual physical completion and returns the retained
// terminal result.
func (l *nodeLifetime) closeResult() error {
	<-l.done
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.result
}
