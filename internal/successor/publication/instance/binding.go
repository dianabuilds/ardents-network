package instance

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// Binding retains the original Instance and independently owned Publication
// reservation. It exports neither private material nor a general signing API.
type Binding struct {
	root         *Root
	generation   *durable.Generation
	credential   publication.Credential
	private      ed25519.PrivateKey
	consumed     bool
	consumption  chan struct{} // One original floor/write operation and its result.
	consumeErr   error
	closed       bool
	closing      chan struct{}
	closeErr     error
	receipt      introduction.Receipt
	record       []byte
	recipients   [2]*Recipient
	revision     uint64
	lifetime     *PublicationLifetime
	observations sync.WaitGroup // Admission and retirement share root.mu.
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
	credential, err := verifyResponse(root.state.response, root.state.request)
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
	current, currentErr := verifyResponse(root.state.response, root.state.request)
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
	root.binding = binding
	root.mu.Unlock()
	return binding, nil
}

// observe reobserves original native custody and the separately owned durable
// reservation outside Instance state. The original borrow keeps both alive
// until the observation returns; the final local gate refuses concurrent revoke.
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
	if err := root.checkOriginal(ctx, lease); err != nil {
		return err
	}
	if err := binding.generation.Check(ctx); err != nil {
		return err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	return binding.checkLocal(ctx)
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
