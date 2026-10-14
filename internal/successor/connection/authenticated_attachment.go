package connection

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// AuthenticatedAttachment retains one actual initial role authentication
// result. It exposes no Application bytes: the logical stream must consume its
// exact initial state once, retaining original authority through all joins.
type AuthenticatedAttachment struct {
	operation                          *executionruntime.Operation
	service                            *instance.ServiceAttachment
	lease                              *instance.ServiceLease
	identity                           wire.LogicalIdentity
	logical, attachment, key, exporter [32]byte
	mu                                 sync.Mutex
	consumed, closed                   bool
	once                               sync.Once
	closeErr                           error
	cleanupErr                         error
	self                               *AuthenticatedAttachment
	role                               wire.Role
	initiator                          *initiatorAttachment
	logicalContext                     context.Context
	cancelLogical                      context.CancelFunc
	recipientRevision                  uint64
	publisherJoin                      *join.Joined // Read-only original Route provenance; Service owns cleanup.
}

// Logical authority survives retirement of an individual TLS Attachment. The
// actual transport separately requires its exact current physical provenance.
func (owner *AuthenticatedAttachment) checkLogical() error {
	if owner == nil || owner.self != owner || owner.operation == nil || owner.logicalContext == nil || owner.cancelLogical == nil {
		return errIntegrity
	}
	if err := owner.logicalContext.Err(); err != nil {
		return err
	}
	switch owner.role {
	case wire.RolePublisher:
		if owner.service == nil || owner.lease == nil || owner.initiator != nil {
			return errIntegrity
		}
		return errors.Join(owner.operation.CheckPublisher(), owner.lease.CheckIdentity(owner.identity, owner.logical))
	case wire.RoleInitiator:
		if owner.initiator == nil || owner.lease != nil || owner.service != nil {
			return errIntegrity
		}
		return errors.Join(owner.logicalContext.Err(), owner.initiator.binding.check(false))
	default:
		return errIntegrity
	}
}

func (owner *AuthenticatedAttachment) check() error {
	if err := owner.checkLogical(); err != nil {
		return err
	}
	if owner.role == wire.RoleInitiator {
		return owner.initiator.check()
	}
	return owner.service.CheckIdentity(owner.identity, owner.logical, owner.attachment)
}

// Final logical checks are local. The caller first observes the original
// owners outside state locks, then enters their original Route barriers.
func (owner *AuthenticatedAttachment) checkLogicalCommit() error {
	if owner == nil || owner.self != owner || owner.operation == nil || owner.logicalContext == nil || owner.cancelLogical == nil {
		return errIntegrity
	}
	if err := owner.logicalContext.Err(); err != nil {
		return err
	}
	if owner.role == wire.RolePublisher {
		if owner.lease == nil || owner.service == nil || owner.initiator != nil {
			return errIntegrity
		}
		return errors.Join(owner.operation.CheckPublisher(), owner.lease.CheckCommitIdentity(owner.identity, owner.logical))
	}
	if owner.role != wire.RoleInitiator || owner.initiator == nil || owner.lease != nil || owner.service != nil {
		return errIntegrity
	}
	binding := owner.initiator.binding
	if binding == nil || binding.self != binding || binding.operation != owner.operation || binding.identity != owner.identity || binding.logical != owner.logical ||
		binding.source == nil || binding.source.self != binding.source || binding.source.config.Operation != owner.operation || binding.source.ctx == nil {
		return errIntegrity
	}
	local, err := owner.operation.ConnectionContext()
	if err != nil {
		return err
	}
	if local == [32]byte{} || local != binding.localContext || local != binding.source.localContext {
		return errIntegrity
	}
	value := binding.proof.Delegation()
	now := time.Now()
	if value.Network != owner.identity.Network || value.Target != owner.identity.Target || value.Instance != owner.identity.Instance || value.Generation != owner.identity.InstanceGeneration ||
		now.Before(value.NotBefore) || !now.Before(value.NotAfter) || !now.Before(time.Unix(owner.identity.WorkSafetyNotAfter, 0)) {
		return errIntegrity
	}
	return errors.Join(owner.operation.CheckReader(), binding.source.ctx.Err())
}

func (owner *AuthenticatedAttachment) context() context.Context {
	return owner.logicalContext
}

// The initial handoff already holds the exact JOIN/Source/Responder barriers.
// Recheck only local seals and the original operation/Instance here; full
// original-owner observations happened before those locks were acquired.
func (owner *AuthenticatedAttachment) checkInitialCommit() error {
	if owner == nil || owner.self != owner || owner.logicalContext == nil || owner.logicalContext.Err() != nil || owner.operation == nil {
		return errIntegrity
	}
	if owner.role == wire.RolePublisher {
		return errors.Join(owner.operation.CheckPublisher(), owner.service.CheckStreamCommit(owner.identity, owner.logical, owner.attachment))
	}
	if owner.role != wire.RoleInitiator || owner.initiator == nil || owner.initiator.self != owner.initiator || owner.initiator.binding == nil ||
		owner.initiator.binding.self != owner.initiator.binding || owner.initiator.binding.operation != owner.operation ||
		owner.initiator.binding.identity != owner.identity || owner.initiator.binding.logical != owner.logical || owner.initiator.binding.attachment != owner.attachment {
		return errIntegrity
	}
	owner.initiator.mu.Lock()
	defer owner.initiator.mu.Unlock()
	if !owner.initiator.initialComplete || owner.initiator.evidence == nil {
		return errIntegrity
	}
	return owner.initiator.checkLocked()
}

// The sealed owner selects its concrete original purpose-scoped transport;
// callers cannot provide an arbitrary successful Reader/Writer here.
type attachmentIO struct {
	owner     *AuthenticatedAttachment
	initiator *initiatorAttachment
	service   *instance.ServiceAttachment
	context   [32]byte
}

// Concrete transport entry performs its own full physical observation before
// borrowing TLS and repeats it after I/O. Keep the separate original logical
// owner observation here without observing that same physical attachment twice
// consecutively. Replacement attachments still retain the original logical owner.
func (attachment attachmentIO) checkIOOwner() error {
	if err := attachment.owner.checkLogical(); err != nil {
		return err
	}
	if attachment.owner.role == wire.RoleInitiator {
		if attachment.initiator == nil || attachment.service != nil {
			return errIntegrity
		}
		return nil
	}
	if attachment.service == nil || attachment.initiator != nil {
		return errIntegrity
	}
	return attachment.service.MatchIdentity(attachment.owner.identity, attachment.owner.logical, attachment.context)
}

func (attachment attachmentIO) Read(body []byte) (int, error) {
	if err := attachment.checkIOOwner(); err != nil {
		return 0, err
	}
	if attachment.owner.role == wire.RoleInitiator {
		return attachment.initiator.Read(body)
	}
	return attachment.service.Read(body)
}

func (attachment attachmentIO) Write(body []byte) (int, error) {
	if err := attachment.checkIOOwner(); err != nil {
		return 0, err
	}
	if attachment.owner.role == wire.RoleInitiator {
		return attachment.initiator.Write(body)
	}
	return attachment.service.Write(body)
}
func (owner *AuthenticatedAttachment) Close() error {
	if owner == nil {
		return nil
	}
	if owner.self != owner {
		return errIntegrity
	}
	owner.once.Do(func() {
		// Synchronous logical denial precedes any mutex wait or physical join.
		if owner.cancelLogical != nil {
			owner.cancelLogical()
		}
		owner.mu.Lock()
		owner.closed = true
		owner.mu.Unlock()
		switch owner.role {
		case wire.RolePublisher:
			owner.closeErr = owner.lease.Close()
			owner.cleanupErr = owner.closeErr
		case wire.RoleInitiator:
			owner.closeErr = owner.initiator.Close()
			owner.cleanupErr = owner.initiator.cleanupErr
		default:
			owner.closeErr = errIntegrity
			owner.cleanupErr = errIntegrity
		}
		owner.mu.Lock()
		clear(owner.key[:])
		owner.mu.Unlock()
		if owner.operation != nil {
			owner.operation.RetainCleanup(owner.cleanupErr)
		}
	})
	return owner.closeErr
}
