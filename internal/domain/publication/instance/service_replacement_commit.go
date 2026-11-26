package instance

import (
	"context"
	"errors"
	"time"
)

// CheckReplacementCommit is the synchronous local gate after actual response
// transfer and independent native/Route observations. It performs no native
// observation or I/O under a consumer's Source generation locks. The original
// recipient cutoff remains a requirement through Connection's final commit,
// even though transferring setup already released its TLS setup-only bound.
func (service *ServiceAttachment) CheckReplacementCommit(ctx context.Context) error {
	if service == nil || service.self != service || ctx == nil || service.opening == nil || service.lease == nil || service.lifetime == nil ||
		service.lifetime.binding == nil || service.lifetime.binding.root == nil || service.ctx == nil || service.lease.ctx == nil ||
		service.lifetime.ctx == nil || service.lease.operation == nil || service.lease.self != service.lease || service.lease.lifetime != service.lifetime {
		return ErrUnavailable
	}
	opening := service.opening
	if opening.recipient == nil || opening.ctx == nil || opening.recipient.binding == nil || opening.recipient.ctx == nil {
		return ErrUnavailable
	}
	root := service.lifetime.binding.root
	opening.mu.Lock()
	defer opening.mu.Unlock()
	root.mu.Lock()
	defer root.mu.Unlock()
	r := opening.recipient
	b := service.lifetime.binding
	now := time.Now()
	if !service.initialComplete || service.generation < 2 || !service.responseStarted || !service.replacementWritten || service.evidence == nil ||
		service.closed || service.lease.closed || service.lifetime.closed || b.closed || root.closed || root.failure != nil || root.binding != b || b.lifetime != service.lifetime ||
		opening.closed || r.closed || r.binding != b || b.recipients[0] != r && b.recipients[1] != r || !now.Before(r.acceptUntil) || !now.Before(opening.expiry) ||
		!now.Before(time.Unix(service.identity.NoNewRecoveryAfter, 0)) || !now.Before(time.Unix(service.identity.WorkSafetyNotAfter, 0)) {
		return ErrUnavailable
	}
	if _, retained := service.lease.attachments[service]; !retained {
		return ErrUnavailable
	}
	if _, retained := service.lifetime.connections[service.lease]; !retained {
		return ErrUnavailable
	}
	return errors.Join(ctx.Err(), opening.ctx.Err(), r.ctx.Err(), service.ctx.Err(), service.lease.ctx.Err(), service.lifetime.ctx.Err(), service.lease.checkCommitLocked())
}
