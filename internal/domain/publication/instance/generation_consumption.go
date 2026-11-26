package instance

import (
	"context"
	"crypto/ed25519"
	"errors"
)

// Consume advances the original Publication floor, then persists a consumed
// redacted Instance. Only this volatile binding retains the live host key.
// One original observer retains native custody through the complete operation;
// Instance state is not held during the other owner's floor write or checks.
func (binding *Binding) Consume(ctx context.Context) error {
	if err := binding.observe(ctx); err != nil {
		return err
	}
	root := binding.root
	root.mu.Lock()
	if err := binding.checkLocal(ctx); err != nil {
		root.mu.Unlock()
		return err
	}
	if binding.consumption != nil {
		done := binding.consumption
		root.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
		root.mu.Lock()
		err := binding.consumeErr
		root.mu.Unlock()
		if err != nil {
			return err
		}
		return binding.observe(ctx)
	}
	binding.consumption = make(chan struct{})
	binding.observations.Add(1)
	root.mu.Unlock()
	err := binding.consumeOriginal(ctx)
	root.mu.Lock()
	binding.consumeErr = err
	close(binding.consumption)
	root.mu.Unlock()
	binding.observations.Done()
	return err
}

func (binding *Binding) consumeOriginal(ctx context.Context) error {
	advanceErr := binding.generation.Advance(ctx)
	root := binding.root
	root.mu.Lock()
	if advanceErr != nil {
		root.failure = errors.Join(root.failure, advanceErr)
		root.state.redact()
		root.mu.Unlock()
		return advanceErr
	}
	// A successful floor burn does not permit a late private state effect.
	// Revoke preserves that burn while refusing the remaining consumption.
	if err := binding.checkLocal(ctx); err != nil {
		root.mu.Unlock()
		return err
	}
	next := root.state.clone()
	next.phase = Consumed
	next.redact()
	if err := root.writeState(ctx, next); err != nil {
		next.erase()
		root.mu.Unlock()
		return err
	}
	binding.private = append(ed25519.PrivateKey(nil), root.state.private...)
	root.replace(next)
	binding.consumed = true
	root.mu.Unlock()
	if err := binding.observe(ctx); err != nil {
		root.mu.Lock()
		root.failure = errors.Join(root.failure, err)
		clear(binding.private)
		binding.private = nil
		root.mu.Unlock()
		return err
	}
	return nil
}
