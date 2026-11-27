package instance

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
)

// Binding retains the original Instance and independently owned Publication
// reservation. It exports neither private material nor a general signing API.
type Binding struct {
	root           *Root
	generation     *durable.Generation
	credential     publication.Credential
	private        ed25519.PrivateKey
	certificateDER []byte // immutable PureEd25519 Service certificate; cached under root.mu (audit DD-7)
	consumed       bool
	consumption    chan struct{} // One original floor/write operation and its result.
	consumeErr     error
	closed         bool
	closing        chan struct{}
	closeErr       error
	receipt        introduction.Receipt
	record         []byte
	recipients     [2]*Recipient
	revision       uint64
	lifetime       *PublicationLifetime
	observations   sync.WaitGroup // Admission and retirement share root.mu.

	// Single-flight coalescing of the physical observation round (audit DD-5
	// family, rework §4.1; the installation startup_check_observation pattern).
	// Concurrent admitted callers share one fresh physical round; the batch is
	// closed before any native read, no positive result is retained across rounds,
	// and each caller keeps its own checkLocal fences. observeMu guards the queue,
	// observeIO serializes the physical round, observeUsers joins it before Close.
	observeMu         sync.Mutex
	observeIO         sync.Mutex
	observeQueue      []*observeRequest
	observing         bool
	observeUsers      sync.WaitGroup
	observeRoundCount atomic.Uint64 // diagnostic: complete physical rounds run (audit DD-1 oracle)
	// observePhysical is the native custody + durable generation round. It is a
	// field so the coalescing oracle can hold a round open deterministically
	// (rework §4.1); Bind installs the genuine original observation.
	observePhysical func(context.Context, rootLease) error
}

// observeRequest is one admitted caller waiting for a complete physical round
// that starts after its own admission. ctx is borrowed only for the duration of
// the shared round; a caller arriving during I/O joins the next round.
type observeRequest struct {
	ctx  context.Context
	done chan error
}

// Bind reconciles through the actual durable owner, never a supplied scalar
// floor. A previously spent Credential is durably redacted before refusal.
func (root *Root) Bind(ctx context.Context, history *durable.Root) (*Binding, error) {
	if root == nil || history == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	root.mu.Lock()
	if err := root.check(ctx); err != nil {
		root.mu.Unlock()
		return nil, err
	}
	if root.binding != nil || root.bindingOpening {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	if root.state.phase == Consumed {
		root.mu.Unlock()
		return nil, ErrSuccessorRequired
	}
	if root.state.phase != Accepted {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	credential, err := publication.VerifyServiceResponse(root.state.response, root.state.request)
	if err != nil || root.state.validate() != nil {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	root.bindingOpening = true
	root.bindingUsers.Add(1)
	root.mu.Unlock()
	defer func() {
		root.mu.Lock()
		root.bindingOpening = false
		root.mu.Unlock()
		root.bindingUsers.Done()
	}()
	generation, err := history.Reserve(ctx, credential)
	root.mu.Lock()
	if currentErr := root.check(ctx); currentErr != nil {
		root.mu.Unlock()
		return nil, errors.Join(currentErr, err, generation.Close())
	}
	current, currentErr := publication.VerifyServiceResponse(root.state.response, root.state.request)
	if root.state.phase != Accepted || root.binding != nil || currentErr != nil || current.Digest() != credential.Digest() {
		root.mu.Unlock()
		return nil, errors.Join(ErrUnavailable, err, currentErr, generation.Close())
	}
	if err != nil {
		if errors.Is(err, durable.ErrConsumed) {
			next := root.state.clone()
			next.phase = Consumed
			next.redact()
			if writeErr := root.writeState(ctx, next); writeErr != nil {
				next.erase()
				root.mu.Unlock()
				return nil, writeErr
			}
			root.replace(next)
			root.mu.Unlock()
			return nil, ErrSuccessorRequired
		}
		root.mu.Unlock()
		return nil, err
	}
	if err = root.check(ctx); err != nil {
		root.mu.Unlock()
		return nil, errors.Join(err, generation.Close())
	}
	binding := &Binding{root: root, generation: generation, credential: credential}
	binding.observePhysical = func(ctx context.Context, lease rootLease) error {
		if err := root.checkOriginal(ctx, lease); err != nil {
			return err
		}
		return generation.Check(ctx)
	}
	root.binding = binding
	root.mu.Unlock()
	return binding, nil
}

// observe reobserves original native custody and the separately owned durable
// reservation outside Instance state. The original borrow keeps both alive
// until the observation returns; the final local gate refuses concurrent revoke.
// Concurrent admitted callers share one fresh physical round (rework §4.1): the
// batch is closed before any native read, no positive result is retained, and
// each caller still runs its own before/after checkLocal fences.
func (binding *Binding) observe(ctx context.Context) error {
	if binding == nil || binding.root == nil || ctx == nil {
		return ErrUnavailable
	}
	root := binding.root
	root.mu.Lock()
	if err := binding.checkLocal(ctx); err != nil {
		root.mu.Unlock()
		return err
	}
	binding.observations.Add(1)
	lease := root.lease
	root.mu.Unlock()
	defer binding.observations.Done()
	request := &observeRequest{ctx: ctx, done: make(chan error, 1)}
	binding.observeMu.Lock()
	binding.observeQueue = append(binding.observeQueue, request)
	if !binding.observing {
		binding.observing = true
		binding.observeUsers.Add(1)
		go binding.observeRounds(lease)
	}
	binding.observeMu.Unlock()
	roundErr := <-request.done
	root.mu.Lock()
	defer root.mu.Unlock()
	// The shared round honors the first admitted caller's context; every caller
	// still applies its own cancellation and local gate, so a canceled caller is
	// refused even when it shared a round with a live caller.
	return errors.Join(roundErr, ctx.Err(), binding.checkLocal(ctx))
}

// observeRounds runs complete physical rounds for queued callers. It takes and
// clears the batch under observeMu before any native read, so neither a later
// request nor an operation following a previous result joins this round. The
// physical round honors the first admitted caller's context; every caller still
// receives the round result and applies its own local gate. observeIO serializes
// rounds; the caller's observations borrow keeps custody alive across them.
func (binding *Binding) observeRounds(lease rootLease) {
	defer binding.observeUsers.Done()
	for {
		binding.observeIO.Lock()
		binding.observeMu.Lock()
		requests := binding.observeQueue
		binding.observeQueue = nil
		binding.observeMu.Unlock()
		binding.observeRoundCount.Add(1)
		err := binding.observePhysical(requests[0].ctx, lease)
		binding.observeIO.Unlock()
		for _, request := range requests {
			request.done <- err
		}
		binding.observeMu.Lock()
		if len(binding.observeQueue) == 0 {
			binding.observing = false
			binding.observeMu.Unlock()
			return
		}
		binding.observeMu.Unlock()
	}
}

// checkLocal follows observe and compares only original local state. It cannot
// replace physical custody or durable floor observations.
func (binding *Binding) checkLocal(ctx context.Context) error {
	root := binding.root
	if ctx == nil || binding.closed || root.closed || root.failure != nil || root.binding != binding ||
		root.state.phase != Accepted && root.state.phase != Consumed || binding.generation == nil {
		return errors.Join(ErrUnavailable, root.failure)
	}
	return binding.generation.CheckCommit(ctx)
}

// Close denies this binding before releasing its original history reservation.
// Root Close joins this same result before erasing state and releasing custody.
func (binding *Binding) Close() error {
	if binding == nil || binding.root == nil {
		return nil
	}
	root := binding.root
	root.mu.Lock()
	if root.binding != binding {
		root.mu.Unlock()
		return ErrUnavailable
	}
	if binding.closing != nil {
		done := binding.closing
		root.mu.Unlock()
		<-done
		root.mu.Lock()
		defer root.mu.Unlock()
		return binding.closeErr
	}
	binding.closed = true
	binding.closing = make(chan struct{})
	lifetime := binding.lifetime
	if lifetime != nil {
		lifetime.cancel()
	}
	root.mu.Unlock()
	if lifetime != nil {
		<-lifetime.done
	}
	// Join any in-flight coalesced physical round before joining the caller
	// borrows; closed is already set under root.mu, so no new round can start.
	binding.observeUsers.Wait()
	binding.observations.Wait()
	root.mu.Lock()
	recipients := binding.recipients
	root.mu.Unlock()
	for _, recipient := range recipients {
		if recipient != nil {
			_ = recipient.Close()
		}
	}
	root.mu.Lock()
	clear(binding.private)
	binding.private = nil
	root.mu.Unlock()
	err := binding.generation.Close()
	if lifetime != nil {
		err = errors.Join(err, lifetime.closeErr)
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	binding.closeErr = err
	close(binding.closing)
	return binding.closeErr
}
