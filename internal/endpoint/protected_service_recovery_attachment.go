//go:build linux

package endpoint

import (
	"context"
	"errors"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

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
