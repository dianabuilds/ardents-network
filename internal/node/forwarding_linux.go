//go:build linux

package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
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
		handle.original.stop()
	}
}
func (handle *Forwarding) Done() <-chan struct{} {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.done
}
func (handle *Forwarding) Close() error {
	if handle == nil {
		return nil
	}
	if handle.original == nil {
		return errors.New("original Node lifetime absent")
	}
	handle.Stop()
	return handle.original.closeResult()
}
func (handle *Forwarding) JoinedOutcome() string {
	if handle == nil || handle.original == nil {
		return "failed"
	}
	return handle.original.joinedOutcome()
}

// JoinedFailureOwners projects fixed local owner categories after actual join.
// It grants no authority and exposes no native error, path or private binding.
func (handle *Forwarding) JoinedFailureOwners() (first, cleanup string) {
	if handle == nil || handle.original == nil {
		return "absent", ""
	}
	return handle.original.joinedFailureOwners()
}

// FirstFailure returns detached original facts without observing or waiting for completion.
func (handle *Forwarding) FirstFailure() *FailureDiagnostic {
	if handle == nil || handle.original == nil {
		return nil
	}
	return handle.original.firstFailure()
}

type forwarding struct {
	nodeLifetime
	config         ForwardingConfig
	credential     *credential
	reservationsMu sync.Mutex
	reservations   map[receiver.Channel]*hosting.Reservation
}

// ListenForwarding acquires original owners and rejects/joins every partial
// startup before returning. No successor binding or automatic restart exists.
func ListenForwarding(ctx context.Context, config ForwardingConfig) (_ *Forwarding, result error) {
	return listenForwarding(ctx, config, createOrReopenForwarding)
}

// ListenForwardingRetained admits a new original invocation using only complete
// existing participation and receiving histories. It supplies no Installation
// restart permission or predecessor completion; composition must prove those
// independently before calling it. All ordinary duty/protection/join checks apply.
func ListenForwardingRetained(ctx context.Context, config ForwardingConfig) (*Forwarding, error) {
	return listenForwarding(ctx, config, retainForwardingHistory)
}

type forwardingOpening uint8

const (
	createOrReopenForwarding forwardingOpening = iota
	retainForwardingHistory
)

func listenForwarding(ctx context.Context, config ForwardingConfig, opening forwardingOpening) (_ *Forwarding, result error) {
	if ctx == nil || config.Authority.Current == nil || !separateRoots(config.ParticipationRoot, config.SpendRoot, config.HostingRoot) {
		return nil, errors.New("original Node configuration invalid")
	}
	f := &forwarding{nodeLifetime: newNodeLifetime(ctx, config.Authority, "Node", config.ParticipationRoot, config.SpendRoot, config.HostingRoot), config: config, reservations: make(map[receiver.Channel]*hosting.Reservation)}
	f.hooks = f
	defer func() {
		if result != nil {
			f.deny(result)
			result = f.joinedStartupFailure(result)
		}
	}()
	var err error
	f.process, err = processresource.Open(f.ctx, processresource.Profile)
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
	retained := opening == retainForwardingHistory
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
	binding := receivingBinding(view, m)
	if err := f.openReceiving(retained, binding, func() (receiving.Observation, error) { return f.observeAdmission(binding) }); err != nil {
		return nil, err
	}
	// All required owners and original participation precede listener effects.
	if err := f.serve(); err != nil {
		return nil, err
	}
	return &Forwarding{original: f}, nil
}

// current crosses owners without holding the Node state mutex, then rechecks
// the original admission latch. Native and producer I/O cannot publish a late
// successful observation for a retired invocation.
func (f *forwarding) current() (network.RuntimeView, error) {
	observation, err := f.checkLocal()
	if err != nil {
		return network.RuntimeView{}, err
	}
	view, err := f.config.Authority.Current()
	failureOwner := "network"
	if err == nil && view.ProfileBinding() != f.config.Authority.Profile {
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
	denied := f.denied
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

// Route's original watcher already calls current every 100 ms and retains the
// full State/duty observation through its physical join. The independent Node
// supervisor checks its own process and producer, including while State I/O is
// held. It grants no RuntimeView; every effect still calls current synchronously.
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
			if _, err := f.checkLocal(); err != nil {
				f.finish()
				return
			}
		}
	}
}

// stop synchronously denies new effects. The original supervisor interrupts and
// joins actual Route users; callers obtain known completion through Done/Close.
func (f *forwarding) stop() {
	if f != nil {
		caller := f.caller
		if caller == nil {
			caller = f.ctx
		}
		f.deny(caller.Err(), "caller")
	}
}

// Forwarding has no separately owned role owner to deny or release: the Route
// receiver is the role and is joined by the shared cleanup.
func (f *forwarding) denyOwner(error) {}
func (f *forwarding) cleanupRole(roleResult error) (func(func(string, error)), error) {
	return nil, roleResult
}
func (f *forwarding) credentialClose() error       { return f.credential.close() }
func (f *forwarding) certificate() tls.Certificate { return f.credential.certificate }
func (f *forwarding) listenerConfig() receiver.ReceiverConfig {
	return receiver.ReceiverConfig{Refill: f.refill}
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
