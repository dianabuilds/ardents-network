//go:build linux || darwin

package node

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/node/internal/credential"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
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

func (handle *Join) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.stop()
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
	return handle.original.closeResult()
}
func (handle *Join) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.joinedOutcome()
}
func (handle *Join) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	return handle.original.joinedFailureOwners()
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Join) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.firstFailure()
}

type joinInvocation struct {
	nodeLifetime
	config     JoinConfig
	credential *credential.Join
}

// ListenJoin joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply accepting pairing authority.
func ListenJoin(ctx context.Context, config JoinConfig) (*Join, error) {
	return listenJoin(ctx, config, false)
}

// ListenJoinRetained requires complete existing participation and receiving
// histories; Installation separately proves original retirement and Startup.
func ListenJoinRetained(ctx context.Context, config JoinConfig) (*Join, error) {
	return listenJoin(ctx, config, true)
}

func listenJoin(ctx context.Context, config JoinConfig, retained bool) (_ *Join, result error) {
	if ctx == nil || config.Authority.Current == nil || !pathcheck.DisjointRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot) {
		return nil, errors.New("original Join configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := &joinInvocation{nodeLifetime: newNodeLifetime(ctx, config.Authority, "Join", config.ParticipationRoot, config.SpendRoot, config.HostingRoot), config: config}
	f.hooks = f
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			result = f.joinedStartupFailure(result)
		}
	}()
	var err error
	f.process, err = processguard.Open(f.ctx, processguard.JoinProfile)
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
	f.credential, err = credential.OpenJoin(config.Certificate, config.PrivateKey)
	if err != nil {
		return nil, err
	}
	if f.credential.PublicKey() != m.PublicKey {
		return nil, errors.New("original Join TLS key differs")
	}
	if err := f.admission(); err != nil {
		return nil, err
	}
	if err := f.openProducer(retained, m); err != nil {
		return nil, err
	}
	view, err := f.current()
	if err != nil {
		return nil, err
	}
	if err := f.openHosting(); err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
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
	return &Join{original: f}, nil
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
	if err := f.credential.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "credential")
	}
	if err := f.producer.Check(); err != nil {
		return network.RuntimeView{}, f.failedObservation(err, "participation")
	}
	view, err := f.config.Authority.Current()
	owner := "network"
	if err == nil && view.ProfileBinding() != f.config.Authority.Profile {
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
	if err := f.credential.Check(); err != nil {
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
	return f.retiredObservation(err, owner)
}

// JOIN has no separately owned role owner to deny or release: the Route receiver
// is the role and is joined by the shared cleanup. Credential custody is fenced
// inside current and released by credentialClose.
func (f *joinInvocation) denyOwner(error) {}
func (f *joinInvocation) cleanupRole(roleResult error) (func(func(string, error)), error) {
	return nil, roleResult
}
func (f *joinInvocation) credentialClose() error       { return f.credential.Close() }
func (f *joinInvocation) certificate() tls.Certificate { return f.credential.TLSCertificate() }
func (f *joinInvocation) listenerConfig() receiver.ReceiverConfig {
	return receiver.ReceiverConfig{}
}

func (f *joinInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || !(class == admission.ForwardClass && c.Hello.Purpose == ardp.PurposeDataJoin) {
		return receiving.Grant{}, errors.New("original Join purpose differs")
	}
	return f.acceptWithReservation(ctx, class, raw, c, f.config.Work, f.config.Termination)
}
