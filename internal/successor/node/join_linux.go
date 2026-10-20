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

// JoinConfig declares one original authenticated Join duty. State
// remains command-owned; Route alone owns matching, credit and physical pairing.
type JoinConfig struct {
	Authority                                 role.Authority
	ParticipationRoot, SpendRoot, HostingRoot string
	Certificate, PrivateKey                   string
	Work, Termination                         hosting.Traffic
}

// Join copies retain one original caller, admission and joined outcome.
// Possession supplies neither an admitted pair nor Service authentication.
type Join struct{ original *joinInvocation }

type joinInvocation struct {
	failure                      failureRecord
	mu                           sync.Mutex
	caller, ctx                  context.Context
	cancel                       context.CancelFunc
	config                       JoinConfig
	denied                       bool
	first, result, cleanupResult error
	firstOwner, cleanupOwner     string
	retired                      error
	done                         chan struct{}
	process                      *processresource.Guard
	producer                     *duty.NodeProducer
	credential                   *joinCredential
	budget                       *hosting.Budget
	receiving                    *receiving.Owner
	role                         *receiver.Receiver
}

// ListenJoin joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply accepting pairing authority.
func ListenJoin(ctx context.Context, config JoinConfig) (_ *Join, result error) {
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot) {
		return nil, errors.New("original Join configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	f := &joinInvocation{failure: failureRecord{started: time.Now()}, caller: ctx, ctx: child, cancel: cancel, config: config, retired: errors.New("original Join admission retired"), done: make(chan struct{})}
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
	f.process, err = processresource.Open(child, processresource.JoinProfile)
	if err != nil {
		return nil, err
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	m, err := config.Authority.Member()
	if err != nil || m.RoleDomain != 2 || m.Subrole != 4 {
		return nil, errors.Join(errors.New("original Join assignment unavailable"), err)
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	f.credential, err = loadJoinCredential(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if string(f.credential.material.key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) {
		return nil, errors.New("original Join TLS key differs")
	}
	if err := f.admission(); err != nil {
		return nil, err
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
	if _, err := f.current(); err != nil {
		return nil, err
	}
	p := view.Profile()
	binding := receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
	f.receiving, err = receiving.Open(config.SpendRoot, binding, func() (receiving.Observation, error) { return f.observeReceiving(binding) })
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
	f.role, err = receiver.Listen(child, receiver.ReceiverConfig{Authority: authority, Certificate: f.credential.material.certificate, Admit: f.admit})
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
	return &Join{original: f}, nil
}

func (f *joinInvocation) admission() error {
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

// Each original process/producer/State observation is made outside the local
// lifetime lock and followed by direct original-caller and admission checks.
func (f *joinInvocation) current() (network.RuntimeView, error) {
	if err := f.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	measurement, err := f.process.Check()
	if err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "process")
	}
	if err := f.credential.check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "credential")
	}
	if err := f.producer.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "participation")
	}
	view, err := f.config.Authority.Current()
	owner := "network"
	if err == nil && view.Profile().ProfileBinding != f.config.Authority.Profile {
		err = errors.New("original Join profile changed")
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
	// Fence the foreign State I/O with genuine original custody and native
	// checks. A live result cannot cover a process/producer/key lost in flight.
	if _, err := f.process.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "process")
	}
	if err := f.credential.check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "credential")
	}
	if err := f.producer.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "participation")
	}
	if err := f.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	return view, nil
}

func (f *joinInvocation) failedObservation(err error, owner string) error {
	if err == f.retired || f.caller.Err() != nil && err == f.caller.Err() || f.ctx.Err() != nil && err == f.ctx.Err() {
		f.deny(f.caller.Err(), "caller")
		return f.retired
	}
	f.deny(err, owner)
	// Dependents receive original denial, not a foreign operational cause
	// mislabeled as their own physical Close failure. Node retains that cause.
	return f.retired
}

func (f *joinInvocation) deny(cause error, owner string) {
	f.mu.Lock()
	f.denied = true
	if f.first == nil && cause != nil && cause != f.retired {
		f.first, f.firstOwner = cause, owner
		f.failure.retain(cause, f.caller, owner)
	}
	f.mu.Unlock()
	f.cancel()
}

func (f *joinInvocation) supervise() {
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

func (f *joinInvocation) cleanup() error {
	var result error
	retain := func(owner string, err error) {
		if err != nil && f.cleanupOwner == "" {
			f.cleanupOwner = owner
		}
		result = errors.Join(result, err)
	}
	if f.role != nil {
		retain("route", originalRetirementResult(f.role.Close(), f.retired))
	}
	// Receiver.Close joins every original reader, writer, pair, callback and
	// reservation before any purpose root, producer or credential is released.
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

func (handle *Join) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.deny(handle.original.caller.Err(), "caller")
	}
}
func (handle *Join) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.done
}
func (handle *Join) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Join lifetime absent")
	}
	handle.Stop()
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result
}
func (handle *Join) JoinedOutcome() string {
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
func (handle *Join) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.firstOwner, f.cleanupOwner
}

func (f *joinInvocation) observeReceiving(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving Join duty changed"), err)
	}
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: admissionFacts(p), Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *joinInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || !(class == admission.ForwardClass && c.Hello.Purpose == ardp.PurposeDataJoin) {
		return receiving.Grant{}, errors.New("original Join purpose differs")
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
func (handle *Join) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	f := handle.original
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failure.snapshot(f.first)
}
