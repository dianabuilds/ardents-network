//go:build linux

package service

import (
	"context"
	"errors"
	"net"
	"sync"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// protectedServiceTransport gives TLS, cancellation and final cleanup one
// physical retirement. A close error remains observable after TLS closes it.
type protectedServiceTransport struct {
	net.Conn
	once sync.Once
	err  error
}

func (transport *protectedServiceTransport) AuthenticatedPeerRetired() bool {
	witness, ok := transport.Conn.(interface{ AuthenticatedPeerRetired() bool })
	return ok && witness.AuthenticatedPeerRetired()
}

func (transport *protectedServiceTransport) Close() error {
	transport.once.Do(func() {
		transport.err = transport.Conn.Close()
		// A retirement attempt after upstream cancellation has already torn
		// down TLS is not a separate cleanup failure. Without this guard the
		// per-stream Join cascade reproduces "text Service transport retirement
		// failed" once per stream and the workload criteria never see a quiet
		// shutdown.
		if transport.err != nil && (errors.Is(transport.err, net.ErrClosed) ||
			transport.err.Error() == "use of closed network connection") {
			transport.err = nil
		}
		if transport.err != nil {
			transport.err = errors.Join(errors.New("text Service transport retirement failed"), transport.err)
		}
	})
	return transport.err
}

// AttachmentOpener returns one already authorized protected Route
// transport and its exact fresh capsule digest. The native Connection owns TLS,
// exporter and retained-continuity verification before committing it.
type AttachmentOpener func(context.Context, nativeconnection.Recovery) (net.Conn, [32]byte, error)

// openInitialAttachment authenticates the first physical transport. The
// caller owns the returned Publisher lease through stream cleanup, including
// when Attachment construction fails.
func openInitialAttachment(binding Binding, lifetime context.Context,
	raw net.Conn, exporterContext [32]byte, client bool, continuity *[32]byte,
) (*nativeconnection.Attachment, *servicepublication.Lease, error) {
	var secured *SecuredAttachment
	var lease *servicepublication.Lease
	var err error
	credential := binding.Credential()
	if client {
		secured, *continuity, err = secureProtectedServiceClient(lifetime, raw, credential, exporterContext, 1)
	} else {
		lease, err = binding.AcquirePublication(lifetime)
		if err != nil {
			return nil, nil, err
		}
		if !binding.MatchesPublication(lease.Current()) {
			return nil, lease, errors.New("text Publisher publication changed")
		}
		secured, *continuity, err = secureProtectedServicePublisher(lifetime, raw, credential, lease, exporterContext, 1)
	}
	if err != nil {
		return nil, lease, err
	}
	// TLS exporter used the fresh Attachment context. Native records must
	// continue to bind the immutable logical context shared by both Endpoints.
	secured.context = binding.Logical()
	first, err := newProtectedServiceAttachment(secured)
	return first, lease, err
}

// openRecoveryAttachment owns one replacement Route transport until TLS
// authenticates it and a native Attachment is constructed.
func openRecoveryAttachment(binding Binding, attempt context.Context,
	request nativeconnection.Recovery, open AttachmentOpener, lease *servicepublication.Lease, client bool,
) (_ *nativeconnection.Attachment, outcome error) {
	if err := binding.ValidateRecovery(request); err != nil {
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
	if err := errors.Join(attempt.Err(), binding.Current()); err != nil {
		return nil, err
	}
	logical := binding.Logical()
	freshContext, err := nativeconnection.ProtectedAttachmentContext(logical, replacementDigest, request.Generation)
	if err != nil {
		return nil, err
	}
	credential := binding.Credential()
	var replacement *SecuredAttachment
	var freshContinuity [32]byte
	if client {
		replacement, freshContinuity, err = secureProtectedServiceClient(attempt, replacementRaw, credential, freshContext, request.Generation)
	} else {
		if lease == nil || !binding.MatchesPublication(lease.Current()) {
			return nil, errors.New("text Publisher publication changed before recovery")
		}
		replacement, freshContinuity, err = secureProtectedServicePublisher(attempt, replacementRaw, credential, lease, freshContext, request.Generation)
	}
	defer clear(freshContinuity[:])
	if err != nil {
		ownedRaw = false // TLS setup owns and closes raw on every failure.
		return nil, err
	}
	ownedRaw = false
	if err := errors.Join(attempt.Err(), binding.Current()); err != nil || !client && !binding.MatchesPublication(lease.Current()) {
		return nil, errors.Join(err, errors.New("text Service authority changed during recovery"), replacement.Close())
	}
	// The exporter was derived from the fresh Attachment context; native
	// Continuity continues to authenticate the immutable logical context.
	replacement.context = logical
	attached, err := newProtectedServiceAttachment(replacement)
	if err != nil {
		return nil, errors.Join(err, replacement.Close())
	}
	return attached, nil
}
