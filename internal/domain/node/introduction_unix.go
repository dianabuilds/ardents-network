//go:build linux || darwin

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/processresource"
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

func (handle *Introduction) Stop() {
	if handle != nil && handle.original != nil {
		handle.original.stop()
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
	return handle.original.closeResult()
}
func (handle *Introduction) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.joinedOutcome()
}
func (handle *Introduction) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	return handle.original.joinedFailureOwners()
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Introduction) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.firstFailure()
}

type introductionInvocation struct {
	nodeLifetime
	config     IntroductionConfig
	credential *credential
	registry   *introduction.Registry
}

// ListenIntroduction joins every rejected original startup. It cannot adopt a
// predecessor, rebind a successor duty, or supply an accepting History callback.
func ListenIntroduction(ctx context.Context, config IntroductionConfig) (_ *Introduction, result error) {
	return listenIntroduction(ctx, config, false)
}

// ListenIntroductionRetained requires complete participation, Receiving and
// Route histories. Platform restart never initializes missing history.
func ListenIntroductionRetained(ctx context.Context, config IntroductionConfig) (*Introduction, error) {
	return listenIntroduction(ctx, config, true)
}

func listenIntroduction(ctx context.Context, config IntroductionConfig, retained bool) (_ *Introduction, result error) {
	if ctx == nil || config.Authority.Current == nil || !pathgrammar.DisjointRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot, config.IntroductionRoot) {
		return nil, errors.New("original Introduction configuration invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := &introductionInvocation{nodeLifetime: newNodeLifetime(ctx, config.Authority, "Introduction", config.ParticipationRoot, config.SpendRoot, config.HostingRoot), config: config}
	f.hooks = f
	defer func() {
		if result != nil {
			f.deny(result, "startup")
			result = f.joinedStartupFailure(result)
		}
	}()
	var err error
	f.process, err = processresource.Open(f.ctx, processresource.IntroductionProfile)
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
	binding := receivingBinding(view, m)
	if err := f.openReceiving(retained, binding, func() (receiving.Observation, error) { return f.observeReceiving(binding) }); err != nil {
		return nil, err
	}
	if retained {
		f.registry, err = introduction.OpenRetainedReceivingRegistry(config.IntroductionRoot, f.historyBinding())
	} else {
		f.registry, err = introduction.OpenReceivingRegistry(config.IntroductionRoot, f.historyBinding(), f.receiving)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.current(); err != nil {
		return nil, err
	}
	if err := f.serve(); err != nil {
		return nil, err
	}
	return &Introduction{original: f}, nil
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
	return f.retiredObservation(err, owner)
}

// denyOwner stops the Registry synchronously. Route's short Stop never waits
// behind History I/O or physical callbacks.
func (f *introductionInvocation) denyOwner(error) {
	if f.registry != nil {
		f.registry.Stop(f.retired)
	}
}

// cleanupRole releases the Registry after Route's physical join and attributes
// its exact operational failure. Only the exact operational leaf is removed from
// the joined release; physical wrappers/causes stay.
func (f *introductionInvocation) cleanupRole(roleResult error) (func(func(string, error)), error) {
	var historyRelease, historyFailure error
	if f.registry != nil {
		joined := f.registry.Close()
		historyFailure = f.registry.Failure()
		historyRelease = originalRetirementResult(joined, historyFailure)
	}
	return func(retain func(string, error)) {
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
	}, originalRetirementResult(roleResult, historyFailure)
}

func (f *introductionInvocation) credentialClose() error       { return f.credential.close() }
func (f *introductionInvocation) certificate() tls.Certificate { return f.credential.certificate }
func (f *introductionInvocation) listenerConfig() receiver.ReceiverConfig {
	return receiver.ReceiverConfig{Introduction: f.registry}
}

func (f *introductionInvocation) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || !(class == admission.RegistrationClass && c.Hello.Purpose == ardp.PurposeIntroduction || class == admission.ControlClass && c.Hello.Purpose == ardp.PurposeSubmission) {
		return receiving.Grant{}, errors.New("original Introduction purpose differs")
	}
	return f.acceptWithReservation(ctx, class, raw, c, f.config.Work, f.config.Termination)
}
