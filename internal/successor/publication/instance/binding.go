package instance

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// Binding retains the original Instance and independently owned Publication
// reservation. It exports neither private material nor a general signing API.
type Binding struct {
	root       *Root
	generation *durable.Generation
	credential publication.Credential
	private    ed25519.PrivateKey
	consumed   bool
	closed     bool
	closing    chan struct{}
	closeErr   error
	receipt    introduction.Receipt
	record     []byte
	recipients [2]*Recipient
	revision   uint64
	lifetime   *PublicationLifetime
}

// Bind reconciles through the actual durable owner, never a supplied scalar
// floor. A previously spent Credential is durably redacted before refusal.
func (root *Root) Bind(ctx context.Context, history *durable.Root) (*Binding, error) {
	if root == nil || history == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return nil, err
	}
	if root.binding != nil {
		return nil, ErrUnavailable
	}
	if root.state.phase == Consumed {
		return nil, ErrSuccessorRequired
	}
	if root.state.phase != Accepted {
		return nil, ErrUnavailable
	}
	credential, err := verifyResponse(root.state.response, root.state.request)
	if err != nil || root.state.validate() != nil {
		return nil, ErrUnavailable
	}
	generation, err := history.Reserve(ctx, credential)
	if err != nil {
		if errors.Is(err, durable.ErrConsumed) {
			next := root.state.clone()
			next.phase = Consumed
			next.redact()
			if writeErr := root.writeState(ctx, next); writeErr != nil {
				next.erase()
				return nil, writeErr
			}
			root.replace(next)
			return nil, ErrSuccessorRequired
		}
		return nil, err
	}
	if err = root.check(ctx); err != nil {
		return nil, errors.Join(err, generation.Close())
	}
	binding := &Binding{root: root, generation: generation, credential: credential}
	root.binding = binding
	return binding, nil
}

func (binding *Binding) check(ctx context.Context) error {
	root := binding.root
	if binding.closed || root.binding != binding || root.state.phase != Accepted && root.state.phase != Consumed {
		return ErrUnavailable
	}
	if err := root.check(ctx); err != nil {
		return err
	}
	value := binding.credential.Delegation()
	now := time.Now()
	if now.Before(value.NotBefore) || !now.Before(value.NotAfter) {
		return ErrUnavailable
	}
	return binding.generation.Check(ctx)
}

// Consume advances the original Publication floor, then persists a consumed
// redacted Instance. Only this volatile binding retains the live host key.
func (binding *Binding) Consume(ctx context.Context) error {
	if binding == nil || binding.root == nil || ctx == nil {
		return ErrUnavailable
	}
	root := binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := binding.check(ctx); err != nil {
		return err
	}
	if binding.consumed {
		return nil
	}
	if err := binding.generation.Advance(ctx); err != nil {
		root.failure = errors.Join(root.failure, err)
		root.state.redact()
		return err
	}
	next := root.state.clone()
	next.phase = Consumed
	next.redact()
	if err := root.writeState(ctx, next); err != nil {
		next.erase()
		return err
	}
	binding.private = append(ed25519.PrivateKey(nil), root.state.private...)
	root.replace(next)
	binding.consumed = true
	if err := binding.check(ctx); err != nil {
		root.failure = errors.Join(root.failure, err)
		clear(binding.private)
		binding.private = nil
		return err
	}
	return nil
}

// Close denies this binding before releasing its original history reservation.
// Root Close joins this same result before erasing state and releasing custody.
func (binding *Binding) Close() error {
	if binding == nil || binding.root == nil {
		return nil
	}
	root := binding.root
	root.mu.Lock()
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
	root.mu.Lock()
	defer root.mu.Unlock()
	binding.closeErr = err
	close(binding.closing)
	return binding.closeErr
}
