package quic

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
)

func TestPeerRetirementCauseRequiresEveryExactLeaf(t *testing.T) {
	peer := &quic.ApplicationError{Remote: true, ErrorCode: 0, ErrorMessage: "role-close"}
	marked := classifyIOError(peer)
	var retained *quic.ApplicationError
	if !errors.Is(marked, peer) || !errors.As(marked, &retained) || retained != peer {
		t.Fatal("native cause identity discarded", marked)
	}
	for _, err := range []error{classifyIOError(peer), fmt.Errorf("physical QUIC: %w", classifyIOError(peer)), errors.Join(classifyIOError(peer), syscall.EPIPE), &net.OpError{Op: "write", Net: "tcp", Err: syscall.ECONNRESET}} {
		if !transport.IsPeerRetirementCause(classifyIOError(err)) {
			t.Fatal("exact peer retirement not classified", err)
		}
	}
	foreign := errors.New("independent close failure")
	for _, err := range []error{nil, net.ErrClosed, errors.New(peer.Error()), &quic.ApplicationError{Remote: false, ErrorCode: 0, ErrorMessage: "role-close"}, &quic.ApplicationError{Remote: true, ErrorCode: 1, ErrorMessage: "role-close"}, &quic.ApplicationError{Remote: true, ErrorCode: 0, ErrorMessage: "unknown"}, errors.Join(peer, foreign), fmt.Errorf("wrapped: %w", errors.Join(foreign, peer))} {
		if transport.IsPeerRetirementCause(classifyIOError(err)) {
			t.Fatal("unknown leaf/phase treated as exact peer retirement", err)
		}
	}
}
