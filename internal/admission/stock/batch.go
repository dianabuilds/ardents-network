//go:build linux

package stock

import (
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// batch is one admitted blind issuance batch. Blinding state and finalized
// stock never leave the owner. The network attempt owns copied request bytes
// only, so revocation can erase secrets under the shared lock while
// cancellation interrupts and joins the transport tree.
type batch struct {
	Refill     bool // Retained internal stock work; never receiver admission authority.
	Prefix     Prefix
	Challenges []credential.ClosedTokenContext
	Selection  client.ClosedBootstrapSelection
	Pending    *credential.PendingClosedTokenBatch
}
