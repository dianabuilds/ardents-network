//go:build linux

package stock

import (
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// batch is one admitted blind issuance batch. Blinding state and finalized
// stock never leave the owner. The network attempt owns copied request bytes
// only, so revocation can erase secrets under the shared lock while
// cancellation interrupts and joins the transport tree.
type batch struct {
	Refill     bool // Retained internal stock work; never receiver admission authority.
	Prefix     Prefix
	Challenges []admissiontoken.ClosedTokenContext
	Selection  client.ClosedBootstrapSelection
	Pending    *admissiontoken.PendingClosedTokenBatch
}
