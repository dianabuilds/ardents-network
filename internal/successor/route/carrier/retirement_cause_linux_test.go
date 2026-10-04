//go:build linux

package carrier

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/quic-go/quic-go"
)

func TestPeerRetirementCauseRequiresEveryExactLeaf(t *testing.T) {
	peer := &quic.ApplicationError{Remote: true, ErrorCode: 0, ErrorMessage: "role-close"}
	for _, err := range []error{peer, fmt.Errorf("physical QUIC: %w", peer), errors.Join(peer, syscall.EPIPE), &net.OpError{Op: "write", Net: "tcp", Err: syscall.ECONNRESET}} {
		if !IsPeerRetirementCause(err) {
			t.Fatal("exact peer retirement not classified", err)
		}
	}
	foreign := errors.New("independent close failure")
	for _, err := range []error{nil, net.ErrClosed, errors.New(peer.Error()), &quic.ApplicationError{Remote: false, ErrorCode: 0, ErrorMessage: "role-close"}, &quic.ApplicationError{Remote: true, ErrorCode: 1, ErrorMessage: "role-close"}, &quic.ApplicationError{Remote: true, ErrorCode: 0, ErrorMessage: "unknown"}, errors.Join(peer, foreign), fmt.Errorf("wrapped: %w", errors.Join(foreign, peer))} {
		if IsPeerRetirementCause(err) {
			t.Fatal("unknown leaf/phase treated as exact peer retirement", err)
		}
	}
}
