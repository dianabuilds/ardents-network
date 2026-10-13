package instance

import (
	"context"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// ServiceLease retains the original Instance for one immutable logical
// Connection, including intervals without a physical Attachment. Possession
// grants no accepting Publication, Execution permission or Application stream.
type ServiceLease struct {
	lifetime  *PublicationLifetime
	operation *executionruntime.Operation
	self      *ServiceLease
	identity  wire.LogicalIdentity
	logical   [32]byte
	// Purpose binding only: Connection owns the secret and recovery decision.
	continuityCommitment [32]byte
	ctx                  context.Context
	cancel               context.CancelFunc
	stopLifetime         func() bool
	lifetimeCallback     chan struct{}
	attachments          map[*ServiceAttachment]struct{}
	closed               bool // Original Instance root mutex owns admission/state.
	physicalFailure      error
	once                 sync.Once
	closeErr             error
}

func (lease *ServiceLease) Context() context.Context {
	if lease == nil {
		return nil
	}
	return lease.ctx
}

// CheckIdentity checks the retained original Instance, context and immutable
// tuple. It neither authenticates another Attachment nor grants recovery.
func (lease *ServiceLease) CheckIdentity(identity wire.LogicalIdentity, logical [32]byte) error {
	if lease == nil || lease.self != lease || lease.lifetime == nil || lease.lifetime.binding == nil || lease.lifetime.binding.root == nil {
		return ErrUnavailable
	}
	root := lease.lifetime.binding.root
	root.mu.Lock()
	if lease.identity != identity || lease.logical != logical {
		root.mu.Unlock()
		return ErrUnavailable
	}
	err := lease.checkCommitLocked()
	root.mu.Unlock()
	if err != nil {
		return err
	}
	if err := lease.lifetime.binding.observe(lease.ctx); err != nil {
		return err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	return lease.checkCommitLocked()
}

// CheckCommitIdentity compares the original live logical lease after a fresh
// CheckIdentity outside consumer locks. It performs no native observation and
// does not depend on a retired physical Attachment during a transport gap.
func (lease *ServiceLease) CheckCommitIdentity(identity wire.LogicalIdentity, logical [32]byte) error {
	if lease == nil || lease.self != lease || lease.lifetime == nil || lease.lifetime.binding == nil || lease.lifetime.binding.root == nil {
		return ErrUnavailable
	}
	root := lease.lifetime.binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if lease.identity != identity || lease.logical != logical {
		return ErrUnavailable
	}
	return lease.checkCommitLocked()
}

// checkCommitLocked checks this same live lease after external native
// observations. The caller holds the original Instance root mutex.
func (lease *ServiceLease) checkCommitLocked() error {
	lifetime := lease.lifetime
	binding := lifetime.binding
	root := binding.root
	if lease.self != lease || lease.closed || lease.ctx == nil || lease.ctx.Err() != nil || lifetime.closed || lifetime.ctx.Err() != nil ||
		binding.closed || root.closed || root.failure != nil || root.binding != binding || binding.lifetime != lifetime ||
		root.state.phase != Accepted && root.state.phase != Consumed || binding.generation == nil {
		return ErrUnavailable
	}
	if _, retained := lifetime.connections[lease]; !retained {
		return ErrUnavailable
	}
	if lease.operation == nil || lifetime.caller != lease.operation.Context() {
		return ErrUnavailable
	}
	value := binding.credential.Delegation()
	now := time.Now()
	if value.Network != lease.identity.Network || value.Target != lease.identity.Target || value.Instance != lease.identity.Instance ||
		value.Generation != lease.identity.InstanceGeneration || now.Before(value.NotBefore) || !now.Before(value.NotAfter) ||
		!now.Before(time.Unix(lease.identity.WorkSafetyNotAfter, 0)) {
		return ErrUnavailable
	}
	if err := lease.operation.CheckPublisher(); err != nil {
		return err
	}
	return binding.generation.CheckCommit(lease.ctx)
}

// Close seals this logical borrow, interrupts and joins all its actual TLS
// Attachments, then returns Instance capacity. Earlier physical cleanup errors
// survive an Attachment gap and concurrent/repeated logical retirement.
func (lease *ServiceLease) Close() error {
	if lease == nil {
		return nil
	}
	if lease.self != lease || lease.lifetime == nil || lease.lifetime.binding == nil || lease.lifetime.binding.root == nil {
		return ErrUnavailable
	}
	lease.once.Do(func() {
		root := lease.lifetime.binding.root
		root.mu.Lock()
		lease.closed = true
		lease.cancel()
		attachments := make([]*ServiceAttachment, 0, len(lease.attachments))
		for attachment := range lease.attachments {
			attachments = append(attachments, attachment)
		}
		root.mu.Unlock()
		for _, attachment := range attachments {
			// The attachment publishes its actual joined result under the same
			// root mutex before returning. Prior retired attachments stay there
			// as failures, rather than being added a second time here.
			_ = attachment.Close()
		}
		if !lease.stopLifetime() {
			<-lease.lifetimeCallback
		}
		root.mu.Lock()
		defer root.mu.Unlock()
		lease.closeErr = lease.physicalFailure
		clear(lease.continuityCommitment[:])
		delete(lease.lifetime.connections, lease)
	})
	return lease.closeErr
}
