package instance

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"testing/synctest"
)

// An original retirement fixture grants no accepting Publication or Instance.
// A copied lifetime must not cancel, close leases or complete the original join.
func TestCopiedPublicationLifetimeCannotRetireOriginal(t *testing.T) {
	root := &Root{}
	binding := &Binding{root: root}
	root.binding = binding
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &PublicationLifetime{binding: binding, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	binding.lifetime = original
	copied := &PublicationLifetime{binding: binding, ctx: ctx, cancel: cancel, done: original.done}
	if !errors.Is(copied.Close(), ErrUnavailable) || ctx.Err() != nil {
		t.Fatal("copied Publication lifetime retired original authority", ctx.Err())
	}
	select {
	case <-original.done:
		t.Fatal("copy supplied original joined completion")
	default:
	}
	if err := original.Close(); err != nil || original.Close() != err || ctx.Err() != context.Canceled {
		t.Fatal("original Publication retirement lost its retained result", err, ctx.Err())
	}
}

// This fixture isolates join and erasure ordering. It supplies no Credential,
// durable reservation, qualified operation, JOIN or successful TLS authority.
func TestServiceRetirementWaitsForOriginalUser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := &Root{}
		binding := &Binding{root: root, private: make(ed25519.PrivateKey, ed25519.PrivateKeySize)}
		root.binding = binding
		ctx, cancel := context.WithCancel(context.Background())
		lifetime := &PublicationLifetime{binding: binding, ctx: ctx, cancel: cancel, done: make(chan struct{})}
		binding.lifetime = lifetime
		logicalCtx, logicalCancel := context.WithCancel(ctx)
		lease := &ServiceLease{lifetime: lifetime, ctx: logicalCtx, cancel: logicalCancel, attachments: make(map[*ServiceAttachment]struct{}), stopLifetime: func() bool { return true }}
		lease.self = lease
		lifetime.connections = map[*ServiceLease]struct{}{lease: {}}
		serviceCtx, serviceCancel := context.WithCancel(logicalCtx)
		watcher := make(chan struct{})
		close(watcher)
		service := &ServiceAttachment{lifetime: lifetime, lease: lease, ctx: serviceCtx, cancel: serviceCancel, watcherDone: watcher, stopLifetime: func() bool { return true }}
		service.self = service
		lease.attachments[service] = struct{}{}
		service.users.Add(1)
		returned := false
		defer func() {
			if !returned {
				service.users.Done()
			}
		}()
		bindingDone := make(chan error, 1)
		lifetimeDone := make(chan error, 1)
		leaseDone := make(chan error, 1)
		serviceDone := make(chan error, 1)
		go func() { bindingDone <- binding.Close() }()
		go func() { lifetimeDone <- lifetime.Close() }()
		synctest.Wait()
		root.mu.Lock()
		_, held := lease.attachments[service]
		retained := binding.closed && lifetime.closed && service.closed && held && len(binding.private) == ed25519.PrivateKeySize
		root.mu.Unlock()
		if !retained || serviceCtx.Err() == nil {
			t.Error("retirement did not interrupt while retaining the original user and Instance key")
		}
		retirements := []struct {
			name  string
			done  <-chan error
			early bool
		}{{name: "Instance", done: bindingDone}, {name: "Publication", done: lifetimeDone}, {name: "logical lease", done: leaseDone}, {name: "Attachment", done: serviceDone}}
		for index := range retirements {
			select {
			case err := <-retirements[index].done:
				retirements[index].early = true
				t.Error(retirements[index].name, "returned before original Service user", err)
			default:
			}
		}
		// Establish the user barrier before racing additional Close calls.
		// A contended sync.Once mutex is not a durable synctest wait: release
		// the user without asking the bubble to quiesce on those mutexes.
		go func() { leaseDone <- lease.Close() }()
		go func() { serviceDone <- service.Close() }()
		service.users.Done()
		returned = true
		synctest.Wait()
		for _, result := range retirements {
			if result.early {
				continue
			}
			select {
			case err := <-result.done:
				if !errors.Is(err, ErrUnavailable) {
					t.Error("missing physical cleanup was not retained", err)
				}
			default:
				t.Error("retirement did not complete after original user joined")
			}
		}
		if len(binding.private) != 0 || len(lease.attachments) != 0 || len(lifetime.connections) != 0 {
			t.Error("joined retirement retained Instance material or Service capacity")
		}
		if !errors.Is(binding.Close(), ErrUnavailable) || !errors.Is(lifetime.Close(), ErrUnavailable) || !errors.Is(service.Close(), ErrUnavailable) {
			t.Error("repeat retirement erased original cleanup failure")
		}
	})
}

// No successful cryptographic or native authority is supplied here. Actual
// Service.Close and parent retirement isolate the logical/physical distinction.
func TestLogicalInstanceLeaseSurvivesAttachmentGap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := &Root{}
		binding := &Binding{root: root, private: make(ed25519.PrivateKey, ed25519.PrivateKeySize)}
		root.binding = binding
		ctx, cancel := context.WithCancel(context.Background())
		lifetime := &PublicationLifetime{binding: binding, ctx: ctx, cancel: cancel, done: make(chan struct{})}
		binding.lifetime = lifetime
		logicalCtx, logicalCancel := context.WithCancel(ctx)
		lease := &ServiceLease{lifetime: lifetime, ctx: logicalCtx, cancel: logicalCancel, attachments: make(map[*ServiceAttachment]struct{}), stopLifetime: func() bool { return true }}
		lease.self = lease
		lifetime.connections = map[*ServiceLease]struct{}{lease: {}}
		serviceCtx, serviceCancel := context.WithCancel(logicalCtx)
		watcher := make(chan struct{})
		close(watcher)
		service := &ServiceAttachment{lifetime: lifetime, lease: lease, ctx: serviceCtx, cancel: serviceCancel, watcherDone: watcher, stopLifetime: func() bool { return true }}
		service.self = service
		lease.attachments[service] = struct{}{}

		foreignService := &ServiceAttachment{self: service, lifetime: lifetime, lease: lease, ctx: serviceCtx, cancel: serviceCancel, watcherDone: watcher, stopLifetime: service.stopLifetime}
		foreignLease := &ServiceLease{self: lease, lifetime: lifetime, ctx: logicalCtx, cancel: logicalCancel, attachments: lease.attachments, stopLifetime: lease.stopLifetime}
		if !errors.Is(foreignService.Close(), ErrUnavailable) || !errors.Is(foreignLease.Close(), ErrUnavailable) || serviceCtx.Err() != nil || logicalCtx.Err() != nil {
			t.Fatal("foreign retirement interrupted the original logical/physical borrow")
		}
		if !errors.Is(service.Close(), ErrUnavailable) {
			t.Fatal("incomplete physical fixture supplied successful JOIN cleanup")
		}
		if serviceCtx.Err() == nil || logicalCtx.Err() != nil || len(binding.private) != ed25519.PrivateKeySize || len(lease.attachments) != 0 || len(lifetime.connections) != 1 {
			t.Fatal("physical retirement canceled or released the original logical Instance lease")
		}
		bindingDone := make(chan error, 1)
		go func() { bindingDone <- binding.Close() }()
		synctest.Wait()
		select {
		case err := <-bindingDone:
			t.Fatal("Instance returned with original logical borrow outstanding", err)
		default:
		}
		if len(binding.private) != ed25519.PrivateKeySize {
			t.Fatal("gap allowed early Instance erasure")
		}
		if !errors.Is(lifetime.Close(), ErrUnavailable) {
			t.Fatal("Publication erased physical failure from retired Attachment")
		}
		synctest.Wait()
		select {
		case err := <-bindingDone:
			if !errors.Is(err, ErrUnavailable) {
				t.Fatal("Instance erased original Attachment cleanup failure", err)
			}
		default:
			t.Fatal("joined logical lease did not return original Instance")
		}
		if len(binding.private) != 0 || len(lifetime.connections) != 0 || logicalCtx.Err() == nil || !errors.Is(lease.Close(), ErrUnavailable) {
			t.Fatal("logical retirement retained capacity/key or erased its original failure")
		}
	})
}
