package stock

import (
	"time"

	admissiontoken "github.com/dianabuilds/ardents-network/internal/domain/admission/token"
)

// batch is one admitted blind issuance batch. Blinding state and finalized
// stock never leave the owner. The network attempt owns copied request bytes
// only. Revocation erases secrets under the Owner lock; the application owns
// cancellation and joining of transport work.
type batch struct {
	Deadline   time.Time
	Refill     bool // Retained internal stock work; never receiver admission authority.
	Bootstrap  bool
	Challenges []admissiontoken.ClosedTokenContext
	Selection  ExchangeBinding
	Pending    *admissiontoken.PendingClosedTokenBatch
}
