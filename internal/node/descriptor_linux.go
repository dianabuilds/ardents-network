//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
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

func (handle *Descriptor) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.stop()
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
	return handle.original.closeResult()
}
func (handle *Descriptor) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.joinedOutcome()
}
func (handle *Descriptor) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	return handle.original.joinedFailureOwners()
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Descriptor) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.firstFailure()
}

type descriptorInvocation struct {
	nodeLifetime
	config     DescriptorConfig
	credential *credential
	store      *reachability.Store
}

// ListenDescriptor joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply an accepting Store callback.
func ListenDescriptor(ctx context.Context, config DescriptorConfig) (*Descriptor, error) {
	return listenDescriptor(ctx, config, false)
}

// ListenDescriptorRetained consumes complete existing histories after separately
// admitted original retirement. Missing participation, spends or Store refuses.
func ListenDescriptorRetained(ctx context.Context, config DescriptorConfig) (*Descriptor, error) {
	return listenDescriptor(ctx, config, true)
}

func listenDescriptor(ctx context.Context, config DescriptorConfig, retained bool) (_ *Descriptor, result error) {
	if ctx == nil || config.Authority.Current == nil || !pathgrammar.DisjointRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, config.DescriptorRoot) {
		return nil, errors.New("original Descriptor configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := &descriptorInvocation{nodeLifetime: newNodeLifetime(ctx, config.Authority, "Descriptor", config.ParticipationRoot, config.SpendRoot, config.HostingRoot), config: config}
	f.hooks = f
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			result = f.joinedStartupFailure(result)
		}
	}()
	var err error
	f.process, err = processresource.Open(f.ctx, processresource.DescriptorProfile)
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
	if _, err := f.currentBase(); err != nil {
		return nil, err
	}
	openStore := reachability.OpenStore
	if retained {
		openStore = reachability.OpenRetainedStore
	}
	f.store, err = openStore(reachability.StoreConfig{Root: config.DescriptorRoot, Network: view.ProfileBinding().Network})
	if err != nil {
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
	return &Descriptor{original: f}, nil
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
	return f.retiredObservation(err, owner)
}

// denyOwner denies the Store synchronously. Its short local denial does not wait
// for a foreign observer or physical publication. Its operational notice cannot
// replace Node's original cause.
func (f *descriptorInvocation) denyOwner(error) {
	if f.store != nil {
		f.store.Deny(f.retired)
	}
}

// cleanupRole releases the Store after Route's physical join and attributes its
// exact operational failure once with its true owner, without filtering
// independent physical Route errors merely because they share an errno or wrapper.
func (f *descriptorInvocation) cleanupRole(roleResult error) (func(func(string, error)), error) {
	var storeRelease, storeFailure error
	if f.store != nil {
		storeRelease = f.store.Close()
		storeFailure = f.store.OperationalFailure()
	}
	return func(retain func(string, error)) {
		if storeFailure != nil {
			f.mu.Lock()
			first := f.first
			if first == nil {
				f.first, f.firstOwner = storeFailure, "reachability"
				f.failure.retain(storeFailure, f.caller, "reachability")
			}
			f.mu.Unlock()
			if first != nil && first != storeFailure {
				retain("reachability", storeFailure)
			}
		}
		retain("reachability", storeRelease)
	}, originalRetirementResult(roleResult, storeFailure)
}

func (f *descriptorInvocation) credentialClose() error       { return f.credential.close() }
func (f *descriptorInvocation) certificate() tls.Certificate { return f.credential.certificate }
func (f *descriptorInvocation) listenerConfig() receiver.ReceiverConfig {
	return receiver.ReceiverConfig{DescriptorStore: f.store}
}

func (f *descriptorInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ControlClass || c.Hello.Purpose != ardp.PurposeReachability {
		return receiving.Grant{}, errors.New("original Descriptor purpose differs")
	}
	return f.acceptWithReservation(ctx, class, raw, c, f.config.Work, f.config.Termination)
}
