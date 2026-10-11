package instance

import (
	"context"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// PublicationLifetime retains the original private binding until the live
// Publisher has joined its registrations and recipients. It exports no key.
type PublicationLifetime struct {
	binding *Binding
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
	closed  bool
}

func (binding *Binding) BeginPublication(ctx context.Context) (*PublicationLifetime, error) {
	if binding == nil || binding.root == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	root := binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := binding.check(ctx); err != nil {
		return nil, err
	}
	if binding.lifetime != nil {
		return nil, ErrUnavailable
	}
	child, cancel := context.WithCancel(ctx)
	lifetime := &PublicationLifetime{binding: binding, ctx: child, cancel: cancel, done: make(chan struct{})}
	binding.lifetime = lifetime
	return lifetime, nil
}

func (lifetime *PublicationLifetime) Context() context.Context { return lifetime.ctx }

func (lifetime *PublicationLifetime) Credential() (publication.Credential, error) {
	if lifetime == nil || lifetime.binding == nil {
		return publication.Credential{}, ErrUnavailable
	}
	root := lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if lifetime.closed || lifetime.ctx.Err() != nil || lifetime.binding.lifetime != lifetime {
		return publication.Credential{}, ErrUnavailable
	}
	if err := lifetime.binding.check(lifetime.ctx); err != nil {
		return publication.Credential{}, err
	}
	return lifetime.binding.credential, nil
}

func (lifetime *PublicationLifetime) Close() {
	if lifetime == nil || lifetime.binding == nil {
		return
	}
	lifetime.once.Do(func() {
		root := lifetime.binding.root
		root.mu.Lock()
		defer root.mu.Unlock()
		lifetime.closed = true
		lifetime.cancel()
		close(lifetime.done)
	})
}
