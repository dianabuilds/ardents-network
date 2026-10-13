package instance

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// PublicationLifetime retains the original private binding until the live
// Publisher has joined its registrations and recipients. It exports no key.
type PublicationLifetime struct {
	binding     *Binding
	ctx         context.Context
	caller      context.Context // Exact original caller, not copied authority.
	cancel      context.CancelFunc
	done        chan struct{}
	once        sync.Once
	closed      bool
	connections map[*ServiceLease]struct{}
	closeErr    error
}

func (binding *Binding) BeginPublication(ctx context.Context) (*PublicationLifetime, error) {
	if binding == nil || binding.root == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	root := binding.root
	if err := binding.observe(ctx); err != nil {
		return nil, err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := binding.checkLocal(ctx); err != nil {
		return nil, err
	}
	if binding.lifetime != nil {
		return nil, ErrUnavailable
	}
	child, cancel := context.WithCancel(ctx)
	lifetime := &PublicationLifetime{binding: binding, ctx: child, caller: ctx, cancel: cancel, done: make(chan struct{}), connections: make(map[*ServiceLease]struct{})}
	binding.lifetime = lifetime
	return lifetime, nil
}

func (lifetime *PublicationLifetime) Context() context.Context { return lifetime.ctx }

// CheckLocal fences the exact original lifetime after fresh Credential/native
// observation. It performs no native read or join under a consumer's state.
func (lifetime *PublicationLifetime) CheckLocal(ctx context.Context) error {
	if lifetime == nil || lifetime.binding == nil || ctx == nil {
		return ErrUnavailable
	}
	root := lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if lifetime.closed || lifetime.binding.lifetime != lifetime {
		return ErrUnavailable
	}
	return errors.Join(ctx.Err(), lifetime.ctx.Err(), lifetime.binding.checkLocal(lifetime.ctx))
}

func (lifetime *PublicationLifetime) Credential() (publication.Credential, error) {
	if lifetime == nil || lifetime.binding == nil {
		return publication.Credential{}, ErrUnavailable
	}
	root := lifetime.binding.root
	if err := lifetime.binding.observe(lifetime.ctx); err != nil {
		return publication.Credential{}, err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if lifetime.closed || lifetime.ctx.Err() != nil || lifetime.binding.lifetime != lifetime {
		return publication.Credential{}, ErrUnavailable
	}
	if err := lifetime.binding.checkLocal(lifetime.ctx); err != nil {
		return publication.Credential{}, err
	}
	return lifetime.binding.credential, nil
}

func (lifetime *PublicationLifetime) Close() error {
	if lifetime == nil || lifetime.binding == nil {
		return nil
	}
	root := lifetime.binding.root
	if root == nil {
		return ErrUnavailable
	}
	root.mu.Lock()
	original := root.binding == lifetime.binding && lifetime.binding.lifetime == lifetime
	root.mu.Unlock()
	if !original {
		return ErrUnavailable
	}
	lifetime.once.Do(func() {
		root.mu.Lock()
		lifetime.closed = true
		lifetime.cancel()
		connections := make([]*ServiceLease, 0, len(lifetime.connections))
		for connection := range lifetime.connections {
			connections = append(connections, connection)
		}
		root.mu.Unlock()
		var result error
		for _, connection := range connections {
			result = errors.Join(result, connection.Close())
		}
		root.mu.Lock()
		defer root.mu.Unlock()
		lifetime.closeErr = result
		close(lifetime.done)
	})
	return lifetime.closeErr
}
