//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// openProtectedServiceInitialAttachment authenticates the first physical
// transport. The caller owns the returned Publisher lease through stream
// cleanup, including when Attachment construction fails.
func (binding *textServiceBinding) openProtectedServiceInitialAttachment(lifetime context.Context,
	raw net.Conn, exporterContext [32]byte, client bool, continuity *[32]byte,
) (*nativeconnection.Attachment, *publication.Lease, error) {
	var secured *securedAttachment
	var lease *publication.Lease
	var err error
	if client {
		secured, *continuity, err = secureProtectedServiceClient(lifetime, raw, binding.credential, exporterContext, 1)
	} else {
		if binding.owner.endpoint.publications == nil {
			return nil, nil, errors.New("text Publisher publication owner unavailable")
		}
		lease, err = binding.owner.endpoint.publications.AcquireAt(lifetime, binding.owner.endpoint.clock().UTC())
		if err != nil {
			return nil, nil, err
		}
		if !binding.matchesPublication(lease.Current()) {
			return nil, lease, errors.New("text Publisher publication changed")
		}
		secured, *continuity, err = secureProtectedServicePublisher(lifetime, raw, binding.credential, lease, exporterContext, 1)
	}
	if err != nil {
		return nil, lease, err
	}
	// TLS exporter used the fresh Attachment context. Native records must
	// continue to bind the immutable logical context shared by both Endpoints.
	secured.context = binding.logical
	first, err := newProtectedServiceAttachment(secured)
	return first, lease, err
}

// openProtectedServiceRecoveryAttachment owns one replacement Route transport
// until TLS authenticates it and a native Attachment is constructed.
func (binding *textServiceBinding) openProtectedServiceRecoveryAttachment(attempt context.Context,
	request nativeconnection.Recovery, open textServiceAttachmentOpener, lease *publication.Lease, client bool,
) (_ *nativeconnection.Attachment, outcome error) {
	if err := binding.validateTextServiceRecovery(request); err != nil {
		return nil, err
	}
	replacementRaw, replacementDigest, err := open(attempt, request)
	if err != nil {
		return nil, err
	}
	if replacementRaw == nil || replacementDigest == [32]byte{} {
		if replacementRaw != nil {
			_ = replacementRaw.Close()
		}
		return nil, errors.New("text Service recovery Attachment is incomplete")
	}
	ownedRaw := true
	defer func() {
		if ownedRaw {
			outcome = errors.Join(outcome, replacementRaw.Close())
		}
	}()
	if err := errors.Join(attempt.Err(), binding.current()); err != nil {
		return nil, err
	}
	freshContext, err := nativeconnection.ProtectedAttachmentContext(binding.logical, replacementDigest, request.Generation)
	if err != nil {
		return nil, err
	}
	var replacement *securedAttachment
	var freshContinuity [32]byte
	if client {
		replacement, freshContinuity, err = secureProtectedServiceClient(attempt, replacementRaw, binding.credential, freshContext, request.Generation)
	} else {
		if lease == nil || !binding.matchesPublication(lease.Current()) {
			return nil, errors.New("text Publisher publication changed before recovery")
		}
		replacement, freshContinuity, err = secureProtectedServicePublisher(attempt, replacementRaw, binding.credential, lease, freshContext, request.Generation)
	}
	defer clear(freshContinuity[:])
	if err != nil {
		ownedRaw = false // TLS setup owns and closes raw on every failure.
		return nil, err
	}
	ownedRaw = false
	if err := errors.Join(attempt.Err(), binding.current()); err != nil || !client && !binding.matchesPublication(lease.Current()) {
		replacement.close()
		return nil, errors.Join(err, errors.New("text Service authority changed during recovery"))
	}
	// The exporter was derived from the fresh Attachment context; native
	// Continuity continues to authenticate the immutable logical context.
	replacement.context = binding.logical
	attached, err := newProtectedServiceAttachment(replacement)
	if err != nil {
		replacement.close()
		return nil, err
	}
	return attached, nil
}
