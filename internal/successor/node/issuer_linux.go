//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	issuing "github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// IssuerConfig declares one original authenticated duty and prepared issuing
// roots. State stays command-owned until Done; private RSA stays in Admission.
type IssuerConfig struct {
	Authority                                 role.Authority
	ParticipationRoot, SpendRoot, HostingRoot string
	Certificate, PrivateKey                   string
	Issuance                                  issuing.Plan
	Work, BootstrapWork, Termination          hosting.Traffic
}

// Issuer copies share original admission, physical users and terminal result.
// Done closes only after Route, issuing borrowers and all acquired owners join.
type Issuer struct{ original *issuerInvocation }

type issuerInvocation struct {
	mu                           sync.Mutex
	caller, ctx                  context.Context
	cancel                       context.CancelFunc
	config                       IssuerConfig
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
	issuing                      issuing.Owner
	role                         *receiver.Receiver
}

// ListenIssuer joins rejected partial acquisitions before returning. It neither
// initializes issuing roots nor silently replaces the original member/profile.
func ListenIssuer(ctx context.Context, config IssuerConfig) (_ *Issuer, result error) {
	p := config.Issuance
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, p.AdmissionRoot, p.KeyRoot, p.ResultRoot) {
		return nil, errors.New("original Issuer configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	f := &issuerInvocation{caller: ctx, ctx: child, cancel: cancel, config: config, retired: errors.New("original Issuer admission retired"), done: make(chan struct{})}
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
	f.process, err = processresource.Open(child, processresource.IssuerProfile)
	if err != nil {
		return nil, err
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	m, err := config.Authority.Member()
	if err != nil || m.RoleDomain != 2 || m.Subrole != 6 || p.KeyBinding.Issuer != m.NodeID || p.KeyBinding.Signer != m.PublicKey {
		return nil, errors.Join(errors.New("original Issuer assignment or signer unavailable"), err)
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	f.credential, err = loadCredential(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if string(f.credential.key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) {
		return nil, errors.New("original Issuer TLS key differs")
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
	var opened issuing.Result
	f.issuing, opened = issuing.OpenCurrent(child, p, f.observeIssuance)
	if opened.Outcome != "opened" {
		return nil, errors.Join(errors.New("original issuing roots unavailable"), f.issuing.Err())
	}
	profile := view.Profile()
	binding := receiving.Receiver{NetworkID: profile.Network, StateGeneration: profile.Generation, StateDigest: profile.EpochDigest, ProfileDigest: profile.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
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
	f.role, err = receiver.Listen(child, receiver.ReceiverConfig{Authority: authority, Certificate: f.credential.certificate, Admit: f.admit, ReserveBootstrap: f.reserveBootstrap, Issue: f.issue})
	if err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	go f.supervise()
	return &Issuer{original: f}, nil
}

// currentBase observes external owners without an issuing callback or Node lock.
// Admission uses this observer; Route additionally checks the retained issuer.
func (f *issuerInvocation) currentBase() (network.RuntimeView, error) {
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
		err = errors.New("original Issuer profile changed")
	}
	if err == nil {
		err = view.MatchDuty(f.config.Authority.Duty, time.Now())
	}
	if err == nil && time.Since(measurement.Completed) > time.Second {
		err, owner = errors.New("original Issuer process measurement expired during authority observation"), "process"
	}
	if err != nil {
		return network.RuntimeView{}, f.failedObservation(err, owner)
	}
	if err := f.admission(); err != nil {
		return network.RuntimeView{}, err
	}
	return view, nil
}

func (f *issuerInvocation) admission() error {
	f.mu.Lock()
	denied := f.denied
	f.mu.Unlock()
	// Check the original caller itself: a derived context's propagation may lag.
	if err := f.caller.Err(); err != nil {
		f.deny(err, "caller")
		return f.retired
	}
	if denied || f.ctx.Err() != nil {
		return f.retired
	}
	return nil
}

func (f *issuerInvocation) current() (network.RuntimeView, error) {
	if _, err := f.currentBase(); err != nil {
		return network.RuntimeView{}, err
	}
	if err := f.issuing.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "issuance")
	}
	// Reobserve process/State after key and journal I/O, before transport effects.
	return f.currentBase()
}

func (f *issuerInvocation) failedObservation(err error, owner string) error {
	if err == f.retired || f.caller.Err() != nil && err == f.caller.Err() || f.ctx.Err() != nil && err == f.ctx.Err() {
		f.deny(f.caller.Err(), "caller")
		return f.retired
	}
	f.deny(err, owner)
	return err
}

func (f *issuerInvocation) deny(cause error, owner string) {
	f.mu.Lock()
	f.denied = true
	if f.first == nil && cause != nil && cause != f.retired {
		f.first, f.firstOwner = cause, owner
	}
	f.mu.Unlock()
	// Never call a foreign owner while holding the Node lifecycle mutex.
	f.issuing.Deny(cause)
	f.cancel()
}

func (f *issuerInvocation) supervise() {
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

func (f *issuerInvocation) cleanup() error {
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
	// The independent background wait never interprets cancellation as joined.
	closed := f.issuing.Close(context.Background())
	for _, c := range closed.Cleanup {
		if c.Phase != "" && c.Outcome != "closed" {
			retain("issuance", f.issuing.Err())
			break
		}
	}
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

func (handle *Issuer) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.deny(nil, "runtime")
	}
}
func (handle *Issuer) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.done
}
func (handle *Issuer) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Issuer lifetime absent")
	}
	handle.Stop()
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.result
}
func (handle *Issuer) JoinedOutcome() string {
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
func (handle *Issuer) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	f := handle.original
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.firstOwner, f.cleanupOwner
}

func (f *issuerInvocation) observeIssuance() (admission.AuthorityFacts, time.Time, error) {
	view, err := f.currentBase()
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	p := view.Profile()
	m, err := view.Member(p.IssuerNodeID, view.ObservedAt())
	if err != nil || m.NodeID != f.config.Authority.Duty.NodeID || m.RoleDomain != 2 || m.Subrole != 6 || m.DutyGeneration != p.IssuerDutyGeneration || m.PublicKey != f.config.Issuance.KeyBinding.Signer {
		return admission.AuthorityFacts{}, time.Time{}, f.failedObservation(errors.Join(errors.New("original issuing member changed"), err), "network")
	}
	return admissionFacts(p), view.ObservedAt(), nil
}

func (f *issuerInvocation) observeReceiving(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving Issuer duty changed"), err)
	}
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: admissionFacts(p), Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *issuerInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ControlClass || c.Hello.Purpose != ardp.PurposeIssuer {
		return receiving.Grant{}, errors.New("original Issuer purpose differs")
	}
	return f.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		held, err := f.reserve(ctx, c.Hello.Deadline, f.config.Work, class.ByteLimit())
		if err != nil {
			return nil, err
		}
		return c.HoldReservation(func() error { return releaseReservation(held) })
	})
}
func (f *issuerInvocation) reserve(ctx context.Context, end time.Time, work hosting.Traffic, limit uint64) (*hosting.Reservation, error) {
	if _, err := f.current(); err != nil {
		return nil, err
	}
	held, err := f.budget.Reserve(ctx, hosting.ReservationRequest{Work: work, Termination: f.config.Termination, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
	if err != nil {
		return nil, err
	}
	if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
		return nil, errors.Join(err, releaseReservation(held))
	}
	if _, err := f.current(); err != nil {
		return nil, errors.Join(err, releaseReservation(held))
	}
	return held, nil
}
func (f *issuerInvocation) reserveBootstrap(ctx context.Context, end time.Time) (func() error, error) {
	held, err := f.reserve(ctx, end, f.config.BootstrapWork, bootstrap.LaneBytes)
	if err != nil {
		return nil, err
	}
	return func() error { return releaseReservation(held) }, nil
}
func (f *issuerInvocation) issue(ctx context.Context, raw []byte, bootstrap bool) ([]byte, error) {
	kind := quota.Admitted
	if bootstrap {
		kind = quota.Bootstrap
	}
	r := f.issuing.IssueCurrent(ctx, raw, kind)
	if err := f.issuing.Err(); err != nil {
		_ = f.failedObservation(err, "issuance")
	}
	if r.Response == nil {
		return nil, errors.New("original Issuer request refused")
	}
	if _, err := f.current(); err != nil {
		clear(r.Response)
		return nil, err
	}
	return r.Response, nil
}
