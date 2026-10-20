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
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// IntroductionConfig declares one original authenticated Introduction duty. State
// remains command-owned; Route alone owns slot, delivery and durable-floor policy.
type IntroductionConfig struct {
	Authority                                                   role.Authority
	ParticipationRoot, SpendRoot, HostingRoot, IntroductionRoot string
	Certificate, PrivateKey                                     string
	Work, Termination                                           hosting.Traffic
}

// Introduction copies retain one original caller, admission and joined outcome.
// Possession of a handle supplies neither an accepting registration nor Service readiness.
type Introduction struct{ original *introductionInvocation }

type introductionInvocation struct {
	failure                      failureRecord
	mu                           sync.Mutex
	caller, ctx                  context.Context
	cancel                       context.CancelFunc
	config                       IntroductionConfig
	denied                       bool
	first, result, cleanupResult error
	firstOwner, cleanupOwner     string
	retired                      error
	done                         chan struct{}
	process                      *processresource.Guard
	producer                     *duty.NodeProducer
	credential                   *credential
	budget                       *hosting.Budget
	receiving                    *receiving.Owner
	registry                     *introduction.Registry
	role                         *receiver.Receiver
}

// ListenIntroduction joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply an accepting History callback.
func ListenIntroduction(ctx context.Context, config IntroductionConfig) (_ *Introduction, result error) {
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, config.IntroductionRoot) {
		return nil, errors.New("original Introduction configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	f := &introductionInvocation{failure: failureRecord{started: time.Now()}, caller: ctx, ctx: child, cancel: cancel, config: config, retired: errors.New("original Introduction admission retired"), done: make(chan struct{})}
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			f.mu.Lock()
			first := f.first
			f.mu.Unlock()
			result = errors.Join(first, originalRetirementResult(result, f.retired), f.cleanup())
			result = &startupFailure{cause: result, fact: f.failure.snapshot(f.first)}
		}
	}()
	var err error
	f.process, err = processresource.Open(child, processresource.IntroductionProfile)
	if err != nil {
		return nil, err
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	m, err := config.Authority.Member()
	if err != nil || m.RoleDomain != 4 || m.Subrole != 3 {
		return nil, errors.Join(errors.New("original Introduction assignment unavailable"), err)
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	f.credential, err = loadCredential(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if string(f.credential.key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) {
		return nil, errors.New("original Introduction TLS key differs")
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	f.producer, err = duty.OpenNodeProducer(child, duty.Config{Root: config.ParticipationRoot, Clock: time.Now, Create: true}, duty.NodeParticipation{Identity: m.NodeID, FamilyID: m.FamilyID, Assignment: m.Assignment, EpochUntil: config.Authority.Duty.Epoch.ValidUntil, RecordUntil: m.NotAfter()})
	if err != nil {
		return nil, err
	}
	view, err := f.currentBase()
	if err != nil {
		return nil, err
	}
	f.budget, err = hosting.Open(config.HostingRoot)
	if err != nil {
		return nil, err
	}
	if _, err := f.currentBase(); err != nil {
		return nil, err
	}
	p := view.Profile()
	binding := receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
	f.receiving, err = receiving.Open(config.SpendRoot, binding, func() (receiving.Observation, error) { return f.observeReceiving(binding) })
	if err != nil {
		return nil, err
	}
	f.registry, err = introduction.OpenReceivingRegistry(config.IntroductionRoot, f.historyBinding(), f.receiving)
	if err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	if err := f.producer.Live(); err != nil {
		return nil, err
	}
	authority := config.Authority
	authority.Current = f.current
	f.role, err = receiver.Listen(child, receiver.ReceiverConfig{Authority: authority, Certificate: f.credential.certificate, Admit: f.admit, Introduction: f.registry})
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
	return &Introduction{original: f}, nil
}

func (f *introductionInvocation) admission() error {
	f.mu.Lock()
	denied := f.denied
	f.mu.Unlock()
	if err := f.caller.Err(); err != nil {
		f.deny(err, "caller")
		return f.retired
	}
	if denied || f.ctx.Err() != nil {
		return f.retired
	}
	return nil
}

// Base observations never call Registry, History or their borrowers. Each
// original native/producer/State observation is followed by direct caller checks.
func (f *introductionInvocation) currentBase() (network.RuntimeView, error) {
	if err := f.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	measurement, err := f.process.Check()
	if err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "process")
	}
	if err := f.producer.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "participation")
	}
	view, err := f.config.Authority.Current()
	owner := "network"
	if err == nil && view.Profile().ProfileBinding != f.config.Authority.Profile {
		err = errors.New("original Introduction profile changed")
	}
	if err == nil {
		err = view.MatchDuty(f.config.Authority.Duty, time.Now())
	}
	if err == nil && time.Since(measurement.Completed) > time.Second {
		err = expiredMeasurement(measurement)
		owner = "process"
	}
	if err != nil {
		return network.RuntimeView{}, f.failedObservation(err, owner)
	}
	if err := f.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	return view, nil
}

func (f *introductionInvocation) historyBinding() introduction.Binding {
	return introduction.Binding{NetworkID: f.config.Authority.Profile.Network, ProfileDigest: f.config.Authority.Profile.Digest, ReceiverNodeID: f.config.Authority.Duty.NodeID, ReceiverDutyGeneration: f.config.Authority.Duty.RecordGeneration}
}

func (f *introductionInvocation) current() (network.RuntimeView, error) {
	view, err := f.currentBase()
	if err != nil {
		return network.RuntimeView{}, err
	}
	// Receiving construction can observe the base before Registry acquisition.
	// Registry is immutable and ready before any listener or asynchronous user.
	if f.registry == nil {
		return view, nil
	}
	if err := f.registry.Check(f.historyBinding()); err != nil {
		if original := f.registry.Failure(); original != nil {
			return network.RuntimeView{}, f.failedObservation(original, "introduction")
		}
		if f.admission() != nil {
			return network.RuntimeView{}, f.retired
		}
		return network.RuntimeView{}, f.failedObservation(err, "introduction")
	}
	return f.currentBase()
}

func (f *introductionInvocation) failedObservation(err error, owner string) error {
	if err == f.retired || f.caller.Err() != nil && err == f.caller.Err() || f.ctx.Err() != nil && err == f.ctx.Err() {
		f.deny(f.caller.Err(), "caller")
		return f.retired
	}
	f.deny(err, owner)
	// Dependents receive original denial, not a foreign operational cause
	// mislabeled as their own physical Close failure. Node retains that cause.
	return f.retired
}

func (f *introductionInvocation) deny(cause error, owner string) {
	f.mu.Lock()
	f.denied = true
	if f.first == nil && cause != nil && cause != f.retired {
		f.first, f.firstOwner = cause, owner
		f.failure.retain(cause, f.caller, owner)
	}
	f.mu.Unlock()
	// Route's short Stop never waits behind History I/O or physical callbacks.
	if f.registry != nil {
		f.registry.Stop(f.retired)
	}
	f.cancel()
}

func (f *introductionInvocation) supervise() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-f.ctx.Done():
			f.deny(f.caller.Err(), "caller")
		case <-f.role.Done():
			f.deny(nil, "route")
		case <-ticker.C:
			if _, err := f.current(); err == nil {
				continue
			}
		}
		cleanup := f.cleanup()
		f.mu.Lock()
		f.failure.retainCleanup(cleanup, f.caller, f.cleanupOwner)
		f.cleanupResult, f.result = cleanup, errors.Join(f.first, cleanup)
		f.mu.Unlock()
		close(f.done)
		return
	}
}

func (f *introductionInvocation) cleanup() error {
	var result error
	retain := func(owner string, err error) {
		if err != nil && f.cleanupOwner == "" {
			f.cleanupOwner = owner
		}
		result = errors.Join(result, err)
	}
	var roleResult error
	if f.role != nil {
		roleResult = originalRetirementResult(f.role.Close(), f.retired)
	}
	// Route.Close physically joins readers/writers/callbacks before any purpose
	// root, reservation, producer or TLS material is returned or erased.
	var historyRelease, historyFailure error
	if f.registry != nil {
		joined := f.registry.Close()
		historyFailure = f.registry.Failure()
		// Remove only the exact operational leaf; physical wrappers/causes stay.
		historyRelease = originalRetirementResult(joined, historyFailure)
	}
	retain("route", originalRetirementResult(roleResult, historyFailure))
	if historyFailure != nil {
		f.mu.Lock()
		first := f.first
		if first == nil {
			f.first, f.firstOwner = historyFailure, "introduction"
			f.failure.retain(historyFailure, f.caller, "introduction")
		}
		f.mu.Unlock()
		if first != nil && first != historyFailure {
			retain("introduction", historyFailure)
		}
	}
	retain("introduction", historyRelease)
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

func (handle *Introduction) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.deny(handle.original.caller.Err(), "caller")
	}
}
func (handle *Introduction) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.done
}
func (handle *Introduction) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Introduction lifetime absent")
	}
	handle.Stop()
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result
}
func (handle *Introduction) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupResult != nil || f.first != nil && f.first != context.Canceled {
		return "failed"
	}
	return "canceled"
}
func (handle *Introduction) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.firstOwner, f.cleanupOwner
}

func (f *introductionInvocation) observeReceiving(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving Introduction duty changed"), err)
	}
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: admissionFacts(p), Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *introductionInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || !(class == admission.RegistrationClass && c.Hello.Purpose == ardp.PurposeIntroduction || class == admission.ControlClass && c.Hello.Purpose == ardp.PurposeSubmission) {
		return receiving.Grant{}, errors.New("original Introduction purpose differs")
	}
	return f.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		if _, err := f.current(); err != nil {
			return nil, err
		}
		held, err := f.budget.Reserve(ctx, hosting.ReservationRequest{Work: f.config.Work, Termination: f.config.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
		if err != nil {
			return nil, err
		}
		limit := class.ByteLimit()
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			return nil, errors.Join(err, releaseReservation(held))
		}
		if _, err := f.current(); err != nil {
			return nil, errors.Join(err, releaseReservation(held))
		}
		return c.HoldReservation(func() error { return releaseReservation(held) })
	})
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Introduction) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	f := handle.original
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failure.snapshot(f.first)
}
