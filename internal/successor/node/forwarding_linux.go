//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// ForwardingConfig identifies exactly one authenticated original duty. Current
// must be the genuine opened new State owner. State remains command-owned and
// is closed only after this original role's joined completion.
type ForwardingConfig struct {
	Authority                                 role.Authority
	ParticipationRoot, SpendRoot, HostingRoot string
	Certificate, PrivateKey                   string
	Work, Termination                         hosting.Traffic
}

// Forwarding retains its original physical users. Done closes only after Route
// join and purpose/Hosting/producer/process/key cleanup, including failed cleanup.
type Forwarding struct{ original *forwarding }

// Copies share the original admission, resources and retained terminal result.
// Neither a copied handle nor a zero value constructs another invocation.
func (handle *Forwarding) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.Stop()
	}
}
func (handle *Forwarding) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.Done()
}
func (handle *Forwarding) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Node lifetime absent")
	}
	return handle.original.Close()
}
func (handle *Forwarding) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.JoinedOutcome()
}

// JoinedFailureOwners projects fixed local owner categories after actual join.
// It grants no authority and exposes no native error, path or private binding.
func (handle *Forwarding) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.firstOwner, f.cleanupOwner
}

type forwarding struct {
	failure                  failureRecord
	mu                       sync.Mutex
	caller, ctx              context.Context
	cancel                   context.CancelFunc
	config                   ForwardingConfig
	denied                   bool
	first, result            error
	cleanupResult            error
	retired                  error
	firstOwner, cleanupOwner string
	done                     chan struct{}
	process                  *processresource.Guard
	producer                 *duty.NodeProducer
	credential               *credential
	budget                   *hosting.Budget
	receiving                *receiving.Owner
	role                     *receiver.Receiver
	reservationsMu           sync.Mutex
	reservations             map[receiver.Channel]*hosting.Reservation
}

// ListenForwarding acquires original owners and rejects/joins every partial
// startup before returning. No successor binding or automatic restart exists.
func ListenForwarding(ctx context.Context, config ForwardingConfig) (_ *Forwarding, result error) {
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot) {
		return nil, errors.New("original Node configuration invalid")
	}
	child, cancel := context.WithCancel(ctx)
	f := &forwarding{failure: failureRecord{started: time.Now()}, caller: ctx, ctx: child, cancel: cancel, config: config, retired: errors.New("original Node admission retired"), done: make(chan struct{}), reservations: make(map[receiver.Channel]*hosting.Reservation)}
	defer func() {
		if result != nil {
			f.deny(result)
			f.mu.Lock()
			first := f.first
			f.mu.Unlock()
			// The caller receives the original cause, not the private notice
			// lent to Route while a rejected partial acquisition joins.
			result = errors.Join(first, originalRetirementResult(result, f.retired), f.cleanup())
			result = &startupFailure{cause: result, fact: f.failure.snapshot(f.first)}
		}
	}()
	var err error
	f.process, err = processresource.Open(child, processresource.Profile)
	if err != nil {
		return nil, err
	}
	m, err := config.Authority.Member()
	if err != nil || !((m.RoleDomain == 1 || m.RoleDomain == 3 || m.RoleDomain == 4) && (m.Subrole == 1 || m.Subrole == 2)) {
		return nil, errors.Join(errors.New("original Forwarding assignment unavailable"), err)
	}
	f.credential, err = loadCredential(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if string(f.credential.key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) {
		return nil, errors.New("original Forwarding TLS key differs")
	}
	f.producer, err = duty.OpenNodeProducer(child, duty.Config{Root: config.ParticipationRoot, Clock: time.Now, Create: true}, duty.NodeParticipation{Identity: m.NodeID, FamilyID: m.FamilyID, Assignment: m.Assignment, EpochUntil: config.Authority.Duty.Epoch.ValidUntil, RecordUntil: m.NotAfter()})
	if err != nil {
		return nil, err
	}
	view, err := f.current()
	if err != nil {
		return nil, err
	}
	f.budget, err = hosting.Open(config.HostingRoot)
	if err != nil {
		return nil, err
	}
	p := view.Profile()
	binding := receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
	f.receiving, err = receiving.Open(config.SpendRoot, binding, func() (receiving.Observation, error) { return f.observeAdmission(binding) })
	if err != nil {
		return nil, err
	}
	authority := config.Authority
	authority.Current = f.current
	// All required owners and original participation precede listener effects.
	// A later listener/startup failure joins any acquired physical users before
	// cleanup removes this conservative live producer.
	if err := f.producer.Live(); err != nil {
		return nil, err
	}
	f.role, err = receiver.Listen(child, receiver.ReceiverConfig{Authority: authority, Certificate: f.credential.certificate, Admit: f.admit, Refill: f.refill})
	if err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.failure.serving = true
	f.mu.Unlock()
	go f.supervise()
	return &Forwarding{original: f}, nil
}

// current crosses owners without holding the Node state mutex, then rechecks
// the original admission latch. Native and producer I/O cannot publish a late
// successful observation for a retired invocation.
func (f *forwarding) current() (network.RuntimeView, error) {
	f.mu.Lock()
	denied := f.denied
	f.mu.Unlock()
	if denied || f.ctx.Err() != nil {
		return network.RuntimeView{}, f.retired
	}
	observation, err := f.process.Check()
	if err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "process")
	}
	if err := f.producer.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "participation")
	}
	view, err := f.config.Authority.Current()
	failureOwner := "network"
	if err == nil && view.Profile().ProfileBinding != f.config.Authority.Profile {
		err = errors.New("original Node profile changed")
	}
	if err == nil {
		err = view.MatchDuty(f.config.Authority.Duty, time.Now())
	}
	if err == nil && time.Since(observation.Completed) > time.Second {
		err = expiredMeasurement(observation)
		failureOwner = "process"
	}
	f.mu.Lock()
	denied = f.denied
	f.mu.Unlock()
	if err == nil && (denied || f.ctx.Err() != nil) {
		err = f.retired
	}
	if err != nil {
		f.deny(err, failureOwner)
	}
	return view, err
}

// A mechanism using this exact invocation's caller may return its bare context
// error while cancellation races the final observation. Preserve that first
// error locally, but lend the unique retirement notice to the Route watcher.
// A foreign error, joined cleanup cause or physical wrapper is never replaced.
func (f *forwarding) failedObservation(cause error, owner string) error {
	callerError := f.ctx.Err()
	ownedNotice := callerError != nil && cause == callerError
	f.deny(cause, owner)
	if ownedNotice {
		return f.retired
	}
	return cause
}

func (f *forwarding) deny(cause error, owners ...string) {
	if cause == f.retired {
		cause = f.ctx.Err()
	}
	f.mu.Lock()
	f.denied = true
	if f.first == nil && cause != nil {
		f.first = cause
		f.firstOwner = "runtime"
		if len(owners) != 0 {
			f.firstOwner = owners[0]
		}
		if cause == context.Canceled {
			f.firstOwner = "caller"
		}
		f.failure.retain(cause, f.ctx, f.firstOwner)
	}
	f.mu.Unlock()
	f.cancel()
}

func (f *forwarding) supervise() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-f.ctx.Done():
			f.deny(f.ctx.Err())
			f.finish()
			return
		case <-f.role.Done():
			f.deny(nil)
			f.finish()
			return
		case <-ticker.C:
			if _, err := f.current(); err != nil {
				f.finish()
				return
			}
		}
	}
}
func (f *forwarding) finish() {
	result := f.cleanup()
	f.mu.Lock()
	f.failure.retainCleanup(result, f.ctx, f.cleanupOwner)
	f.cleanupResult = result
	f.result = errors.Join(f.first, result)
	f.mu.Unlock()
	close(f.done)
}

// JoinedOutcome is a bounded diagnostic after actual completion. It never
// grants admission and never classifies a physical cleanup error as cancellation.
func (f *forwarding) JoinedOutcome() string {
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupResult != nil || f.first != nil && f.first != context.Canceled {
		return "failed"
	}
	return "canceled"
}
func (f *forwarding) cleanup() error {
	var result error
	retain := func(owner string, cause error) {
		if cause != nil && f.cleanupOwner == "" {
			f.cleanupOwner = owner
		}
		result = errors.Join(result, cause)
	}
	if f.role != nil {
		retain("route", originalRetirementResult(f.role.Close(), f.retired))
	}
	// Receiver.Close returns only after physical join, even when its retained
	// physical result fails. No bounded waiting context releases these owners.
	if f.receiving != nil {
		retain("admission", f.receiving.Close())
	}
	if f.budget != nil {
		retain("hosting", f.budget.Close())
	}
	if f.producer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		retain("participation", f.producer.Close(ctx))
		cancel()
	}
	retain("process", f.process.Close())
	retain("credential", f.credential.close())
	return result
}

// Route's watcher can observe this invocation's own already-denied admission
// while SIGINT cleanup is joining. Only that exact unwrapped local notice is
// not a cleanup failure. Foreign causes and single-error physical wrappers are
// retained, even if they wrap cancellation or this notice. This does not infer
// successful physical completion: Receiver.Close has already joined its users.
func originalRetirementResult(result, notice error) error {
	retained, _ := removeOriginalRetirementNotice(result, notice)
	return retained
}

// Preserve every unchanged error node: another owner may subsequently remove
// its exact retained operational subtree while keeping physical wrappers.
func removeOriginalRetirementNotice(result, notice error) (error, bool) {
	if result == notice {
		return nil, result != nil
	}
	if joined, ok := result.(interface{ Unwrap() []error }); ok {
		var retained []error
		changed := false
		for _, cause := range joined.Unwrap() {
			next, removed := removeOriginalRetirementNotice(cause, notice)
			retained = append(retained, next)
			changed = changed || removed
		}
		if changed {
			return errors.Join(retained...), true
		}
	}
	return result, false
}

// Stop synchronously denies new effects. The original supervisor interrupts and
// joins actual Route users; callers obtain known completion through Done/Close.
func (f *forwarding) Stop() {
	if f != nil {
		caller := f.caller
		if caller == nil {
			caller = f.ctx
		}
		f.deny(caller.Err(), "caller")
	}
}
func (f *forwarding) Done() <-chan struct{} { return f.done }
func (f *forwarding) Close() error {
	if f == nil {
		return nil
	}
	f.Stop()
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result
}

func (f *forwarding) observeAdmission(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving duty changed"), err)
	}
	facts := admissionFacts(p)
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: facts, Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *forwarding) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ForwardClass {
		return receiving.Grant{}, errors.Join(errors.New("original Forwarding purpose differs"), err)
	}
	return f.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		held, err := f.budget.Reserve(ctx, hosting.ReservationRequest{Work: f.config.Work, Termination: f.config.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
		if err != nil {
			return nil, err
		}
		limit := class.ByteLimit()
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			// No physical borrower or receiving spend exists yet. An operator's
			// smaller envelope cannot back the token's complete allowance.
			return nil, errors.Join(err, releaseReservation(held))
		}
		f.reservationsMu.Lock()
		f.reservations[c] = held
		f.reservationsMu.Unlock()
		return c.HoldReservation(func() error {
			f.reservationsMu.Lock()
			delete(f.reservations, c)
			f.reservationsMu.Unlock()
			return releaseReservation(held)
		})
	})
}
func releaseReservation(held *hosting.Reservation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return held.Release(ctx)
}
func (f *forwarding) refill(ctx context.Context, c receiver.Channel, original receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
	limit := admission.ForwardClass.ByteLimit()
	if c.Hello.Purpose != ardp.PurposeForwarding || remaining == 0 || remaining >= limit {
		return receiving.Grant{}, errors.New("original Forwarding refill unavailable")
	}
	f.reservationsMu.Lock()
	held := f.reservations[c]
	f.reservationsMu.Unlock()
	return f.receiving.Refill(ctx, original, remaining, raw, func() (func() error, error) {
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			return nil, err
		}
		delta := limit - remaining
		additional, err := f.budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
		if err != nil {
			return nil, err
		}
		return c.HoldReservation(func() error { return releaseReservation(additional) })
	})
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Forwarding) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	f := handle.original
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failure.snapshot(f.first)
}
