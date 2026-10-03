package stock

import (
	admissiontoken "github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"time"
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
