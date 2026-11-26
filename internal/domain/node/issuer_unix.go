//go:build linux || darwin

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	issuing "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/bootstrap"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
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

func (handle *Issuer) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.stop()
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
	return handle.original.closeResult()
}
func (handle *Issuer) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.joinedOutcome()
}
func (handle *Issuer) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	return handle.original.joinedFailureOwners()
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Issuer) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.firstFailure()
}

type issuerInvocation struct {
	nodeLifetime
	config     IssuerConfig
	credential *credential
	issuing    issuing.Owner
}

// ListenIssuer joins rejected partial acquisitions before returning. It neither
// initializes issuing roots nor silently replaces the original member/profile.
func ListenIssuer(ctx context.Context, config IssuerConfig) (_ *Issuer, result error) {
	return listenIssuer(ctx, config, false)
}

// ListenIssuerRetained keeps quota/results, participation and Receiving history
// with their original owners. Missing state cannot become fresh issuance rights.
func ListenIssuerRetained(ctx context.Context, config IssuerConfig) (*Issuer, error) {
	return listenIssuer(ctx, config, true)
}

func listenIssuer(ctx context.Context, config IssuerConfig, retained bool) (_ *Issuer, result error) {
	p := config.Issuance
	if ctx == nil || config.Authority.Current == nil || !pathgrammar.DisjointRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, p.AdmissionRoot, p.KeyRoot, p.ResultRoot) {
		return nil, errors.New("original Issuer configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := &issuerInvocation{nodeLifetime: newNodeLifetime(ctx, config.Authority, "Issuer", config.ParticipationRoot, config.SpendRoot, config.HostingRoot), config: config}
	f.hooks = f
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			result = f.joinedStartupFailure(result)
		}
	}()
	var err error
	f.process, err = processresource.Open(f.ctx, processresource.IssuerProfile)
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
	if err := f.openProducer(retained, m); err != nil {
		return nil, err
	}
	view, err := f.currentBase()
	if err != nil {
		return nil, err
	}
	if err := f.openHosting(); err != nil {
		return nil, err
	}
	var opened issuing.Result
	f.issuing, opened = issuing.OpenCurrent(f.ctx, p, f.observeIssuance)
	if opened.Outcome != "opened" {
		return nil, errors.Join(errors.New("original issuing roots unavailable"), f.issuing.Err())
	}
	binding := receivingBinding(view, m)
	if err := f.openReceiving(retained, binding, func() (receiving.Observation, error) { return f.observeReceiving(binding) }); err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	if err := f.serve(); err != nil {
		return nil, err
	}
	return &Issuer{original: f}, nil
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

// failedObservation keeps the Issuer notice policy: a foreign operational cause
// is returned to the caller (not replaced by the retirement notice), while a bare
// caller/cancellation error is lent the retirement notice. Node retains the cause.
func (f *issuerInvocation) failedObservation(err error, owner string) error {
	if err == f.retired || f.caller.Err() != nil && err == f.caller.Err() || f.ctx.Err() != nil && err == f.ctx.Err() {
		f.deny(f.caller.Err(), "caller")
		return f.retired
	}
	f.deny(err, owner)
	return err
}

// denyOwner denies the issuing owner. Never call a foreign owner while holding
// the Node lifecycle mutex; the shared latch releases the lock first. A zero
// issuing owner (startup refused before acquisition) is a safe no-op.
func (f *issuerInvocation) denyOwner(cause error) {
	f.issuing.Deny(cause)
}

// cleanupRole joins the independent issuing background wait after Route join.
// The background wait never interprets cancellation as joined.
func (f *issuerInvocation) cleanupRole(roleResult error) (func(func(string, error)), error) {
	closed := f.issuing.Close(context.Background())
	return func(retain func(string, error)) {
		for _, c := range closed.Cleanup {
			if c.Phase != "" && c.Outcome != "closed" {
				retain("issuance", f.issuing.Err())
				break
			}
		}
	}, roleResult
}

func (f *issuerInvocation) credentialClose() error       { return f.credential.close() }
func (f *issuerInvocation) certificate() tls.Certificate { return f.credential.certificate }
func (f *issuerInvocation) listenerConfig() receiver.ReceiverConfig {
	return receiver.ReceiverConfig{ReserveBootstrap: f.reserveBootstrap, Issue: f.issue}
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
		return holdIssuerReservation(c, held)
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
		return nil, errors.Join(err, held.ReleaseJoined())
	}
	if _, err := f.current(); err != nil {
		return nil, errors.Join(err, held.ReleaseJoined())
	}
	return held, nil
}
func (f *issuerInvocation) reserveBootstrap(ctx context.Context, end time.Time) (func() error, error) {
	held, err := f.reserve(ctx, end, f.config.BootstrapWork, bootstrap.LaneBytes)
	if err != nil {
		return nil, err
	}
	return held.ReleaseJoined, nil
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
