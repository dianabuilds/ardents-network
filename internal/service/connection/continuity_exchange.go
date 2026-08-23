package connection

import (
	"context"
	"errors"
	"io"
	"time"
)

const continuityExchangeLimit = 15 * time.Second

// ErrContinuityViolation means that a peer did not prove the exact logical
// connection and fresh TLS attachment selected by the caller.
var ErrContinuityViolation = errors.New("native Service Connection continuity violation")

// ContinuityExchange fixes the local state that must appear in one native
// Continuity record. It deliberately accepts only an io.ReadWriter: TLS and
// Route selection remain outside this protocol owner.
type ContinuityExchange struct {
	Key, Context, ExporterCommitment [32]byte
	Role                             Role
	Generation, SendBase, SendEnd    uint64
	ReceiveNext                      uint64
}

// ContinuityPeer is the verified remote logical-stream state returned by an
// exchange. Nonces are retained solely for the caller's cutover invariants.
type ContinuityPeer struct {
	SendBase, SendEnd, ReceiveNext uint64
	PeerNonce, LocalNonce          [32]byte
}

// ExchangeContinuity performs the fixed role-ordered Continuity exchange and
// verifies the peer record against the immutable attachment facts.
func ExchangeContinuity(ctx context.Context, carrier io.ReadWriter, input ContinuityExchange) (ContinuityPeer, error) {
	if carrier == nil {
		return ContinuityPeer{}, errors.New("native continuity carrier is absent")
	}
	if input.Role != RoleClient && input.Role != RolePublisher {
		return ContinuityPeer{}, errors.New("native continuity role is invalid")
	}
	deadline := time.Now().Add(continuityExchangeLimit)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	if deadlines, ok := carrier.(interface{ SetDeadline(time.Time) error }); ok {
		_ = deadlines.SetDeadline(deadline)
		defer deadlines.SetDeadline(time.Time{})
	}
	local, err := NewContinuity(input.Key, input.Role, input.Generation, input.SendBase, input.SendEnd,
		input.ReceiveNext, input.Context, input.ExporterCommitment)
	if err != nil {
		return ContinuityPeer{}, err
	}
	peerRole := RoleClient
	if input.Role == RoleClient {
		peerRole = RolePublisher
	}
	var record Record
	if input.Role == RoleClient {
		if err := Write(carrier, Record{Continuity: &local}); err != nil {
			return ContinuityPeer{}, err
		}
		record, err = Read(carrier)
	} else {
		record, err = Read(carrier)
		if err == nil {
			err = Write(carrier, Record{Continuity: &local})
		}
	}
	if err != nil || record.Continuity == nil ||
		VerifyContinuity(input.Key, *record.Continuity, peerRole, input.Generation, input.Context, input.ExporterCommitment) != nil {
		return ContinuityPeer{}, ErrContinuityViolation
	}
	peer := record.Continuity
	return ContinuityPeer{SendBase: peer.SendBase, SendEnd: peer.SendEnd, ReceiveNext: peer.ReceiveNext,
		PeerNonce: peer.Nonce, LocalNonce: local.Nonce}, nil
}
