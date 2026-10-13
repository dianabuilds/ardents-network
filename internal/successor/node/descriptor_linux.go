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
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// DescriptorConfig declares one original authenticated Descriptor duty. State
// remains command-owned; Reachability alone owns proof and durable-floor policy.
type DescriptorConfig struct {
	Authority                                                 role.Authority
	ParticipationRoot, SpendRoot, HostingRoot, DescriptorRoot string
	Certificate, PrivateKey                                   string
	Work, Termination                                         hosting.Traffic
}

// Descriptor copies retain one original caller, admission and joined outcome.
// Possession of a handle supplies neither a Descriptor proof nor Service readiness.
type Descriptor struct{ original *descriptorInvocation }

type descriptorInvocation struct {
	mu                           sync.Mutex
	caller, ctx                  context.Context
	cancel                       context.CancelFunc
	config                       DescriptorConfig
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
	store                        *reachability.Store
	role                         *receiver.Receiver
}

// ListenDescriptor joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply an accepting Store callback.
func ListenDescriptor(ctx context.Context, config DescriptorConfig) (_ *Descriptor, result error) {
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, config.DescriptorRoot) {
		return nil, errors.New("original Descriptor configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	f := &descriptorInvocation{caller: ctx, ctx: child, cancel: cancel, config: config, retired: errors.New("original Descriptor admission retired"), done: make(chan struct{})}
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			f.mu.Lock()
			first := f.first
			f.mu.Unlock()
			result = errors.Join(first, originalRetirementResult(result, f.retired), f.cleanup())
		}
	}()
	var err error
	f.process, err = processresource.Open(child, processresource.DescriptorProfile)
	if err != nil {
		return nil, err
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	m, err := config.Authority.Member()
	if err != nil || m.RoleDomain != 2 || m.Subrole != 5 {
		return nil, errors.Join(errors.New("original Descriptor assignment unavailable"), err)
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	f.credential, err = loadCredential(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if string(f.credential.key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) {
		return nil, errors.New("original Descriptor TLS key differs")
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
	f.store, err = reachability.OpenStore(reachability.StoreConfig{Root: config.DescriptorRoot, Network: view.Profile().Network})
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
	f.role, err = receiver.Listen(child, receiver.ReceiverConfig{Authority: authority, Certificate: f.credential.certificate, Admit: f.admit, DescriptorStore: f.store})
	if err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	go f.supervise()
	return &Descriptor{original: f}, nil
}

func (f *descriptorInvocation) admission() error {
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

// Base observations never call Store or an observer supplied to Store. Each
// original native/producer/State observation is followed by direct caller checks.
func (f *descriptorInvocation) currentBase() (network.RuntimeView, error) {
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
		err = errors.New("original Descriptor profile changed")
	}
	if err == nil {
		err = view.MatchDuty(f.config.Authority.Duty, time.Now())
	}
	if err == nil && time.Since(measurement.Completed) > time.Second {
		err = errors.New("original Descriptor process measurement expired during authority observation")
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

func (f *descriptorInvocation) current() (network.RuntimeView, error) {
	if _, err := f.currentBase(); err != nil {
		return network.RuntimeView{}, err
	}
	if err := f.store.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "reachability")
	}
	// Store does native I/O only, with no foreign observer. Reobserve original
	// native process and signed State after that I/O before any Route effect.
	return f.currentBase()
}

func (f *descriptorInvocation) failedObservation(err error, owner string) error {
	if err == f.retired || f.caller.Err() != nil && err == f.caller.Err() || f.ctx.Err() != nil && err == f.ctx.Err() {
		f.deny(f.caller.Err(), "caller")
		return f.retired
	}
	f.deny(err, owner)
	// Dependents receive original denial, not a foreign operational cause
	// mislabeled as their own physical Close failure. Node retains that cause.
	return f.retired
}

func (f *descriptorInvocation) deny(cause error, owner string) {
	f.mu.Lock()
	f.denied = true
	if f.first == nil && cause != nil && cause != f.retired {
		f.first, f.firstOwner = cause, owner
	}
	f.mu.Unlock()
	// Store's short local denial does not wait for a foreign observer or physical
	// publication. Its operational notice cannot replace Node's original cause.
	if f.store != nil {
		f.store.Deny(f.retired)
	}
	f.cancel()
}

func (f *descriptorInvocation) supervise() {
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
		f.cleanupResult, f.result = cleanup, errors.Join(f.first, cleanup)
		f.mu.Unlock()
		close(f.done)
		return
	}
}

func (f *descriptorInvocation) cleanup() error {
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
	var storeRelease, storeFailure error
	if f.store != nil {
		storeRelease = f.store.Close()
		storeFailure = f.store.OperationalFailure()
	}
	// Receiver's terminal result can contain that exact foreign operational
	// cause. Retain it once with its true owner, without filtering independent
	// physical Route errors merely because they share an errno or wrapper.
	retain("route", originalRetirementResult(roleResult, storeFailure))
	if storeFailure != nil {
		f.mu.Lock()
		first := f.first
		if first == nil {
			f.first, f.firstOwner = storeFailure, "reachability"
		}
		f.mu.Unlock()
		if first != nil && first != storeFailure {
			retain("reachability", storeFailure)
		}
	}
	retain("reachability", storeRelease)
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

func (handle *Descriptor) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.deny(nil, "runtime")
	}
}
func (handle *Descriptor) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.done
}
func (handle *Descriptor) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Descriptor lifetime absent")
	}
	handle.Stop()
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result
}
func (handle *Descriptor) JoinedOutcome() string {
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
func (handle *Descriptor) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.firstOwner, f.cleanupOwner
}

func (f *descriptorInvocation) observeReceiving(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving Descriptor duty changed"), err)
	}
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: admissionFacts(p), Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *descriptorInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ControlClass || c.Hello.Purpose != ardp.PurposeReachability {
		return receiving.Grant{}, errors.New("original Descriptor purpose differs")
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
