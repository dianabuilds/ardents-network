package instance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

// Genuine native Instance custody/floor; the incomplete Service fixture below
// isolates join ordering and retains unavailable physical cleanup. It supplies
// no successful JOIN, TLS, qualified Publisher or authenticated Connection.
func TestNativeServiceBorrowJoinsUsersBeforeInstanceErasure(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		root.Close()
		history.Close()
		t.Fatal(err)
	}
	lifetime, err := binding.BeginPublication(context.Background())
	if err != nil {
		binding.Close()
		root.Close()
		history.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { lifetime.Close(); binding.Close(); root.Close(); history.Close() })
	if err := binding.Consume(context.Background()); err != nil {
		t.Fatal(err)
	}
	logicalCtx, logicalCancel := context.WithCancel(lifetime.Context())
	lease := &ServiceLease{lifetime: lifetime, ctx: logicalCtx, cancel: logicalCancel, attachments: make(map[*ServiceAttachment]struct{}), stopLifetime: func() bool { return true }}
	lease.self = lease
	ctx, cancel := context.WithCancel(logicalCtx)
	watcher := make(chan struct{})
	close(watcher)
	service := &ServiceAttachment{lifetime: lifetime, lease: lease, ctx: ctx, cancel: cancel, watcherDone: watcher, stopLifetime: func() bool { return true }}
	service.self = service
	service.users.Add(1)
	returned := false
	defer func() {
		if !returned {
			service.users.Done()
		}
	}()
	root.mu.Lock()
	lifetime.connections[lease] = struct{}{}
	lease.attachments[service] = struct{}{}
	root.mu.Unlock()
	bindingDone := make(chan error, 1)
	go func() { bindingDone <- binding.Close() }()
	select {
	case <-lifetime.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("original binding did not deny Service")
	}
	lifetimeDone := make(chan error, 1)
	go func() { lifetimeDone <- lifetime.Close() }()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Service was not interrupted before join")
	}
	root.mu.Lock()
	retained := len(binding.private) == 64
	root.mu.Unlock()
	if !retained {
		t.Fatal("Instance erased before original TLS user joined")
	}
	select {
	case err := <-bindingDone:
		t.Fatal("binding returned before TLS user", err)
	default:
	}
	select {
	case err := <-lifetimeDone:
		t.Fatal("Publication returned before TLS user", err)
	default:
	}
	service.users.Done()
	returned = true
	select {
	case err := <-lifetimeDone:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatal("unavailable original physical cleanup erased", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Publication did not join original TLS user")
	}
	select {
	case err := <-bindingDone:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatal("binding did not retain original cleanup", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("binding did not join Publication")
	}
	root.mu.Lock()
	retained = len(binding.private) != 0
	root.mu.Unlock()
	if retained {
		t.Fatal("joined Instance retained private material")
	}
	if !errors.Is(lifetime.Close(), ErrUnavailable) || !errors.Is(service.Close(), ErrUnavailable) || !errors.Is(binding.Close(), ErrUnavailable) {
		t.Fatal("repeat Close erased failed original cleanup")
	}
}
