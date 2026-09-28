//go:build linux

package tokens

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Host is the duty context seam for the token owner. The endpoint package
// implements it; every Locked method is called while the shared duty context
// lock is held, exactly as the pre-extraction context methods were.
type Host interface {
	// ProfileLocked returns the live verified State authority profile and its
	// exact current time, or fails closed.
	ProfileLocked() (state.ClosedProfileView, time.Time, error)
	// SurfaceRole maps the context surface to its allocation role.
	SurfaceRole() admission.AllocationRole
	// Fail records a terminal cleanup failure on the context and fails every
	// duty context of the Endpoint generation.
	Fail(err error)
	// Journal returns the exclusive bounded durable token journal.
	Journal() (*tokenjournal.Journal, error)
	// LeaseContext is the authorization lease lifetime bounding issuance.
	LeaseContext() context.Context
	// SelectBootstrapLocked returns the exact current bootstrap selection.
	SelectBootstrapLocked() (client.ClosedBootstrapSelection, error)
	// PrefixCurrent reports whether the prefix is the context's live Source.
	PrefixCurrent(prefix Prefix) bool
}

// Prefix is one exact published Source opening as the token owner may hold it:
// an opaque identity for batch binding plus its transport capabilities. The
// endpoint package supplies the stable per-handle implementation; identity
// comparisons require pointer-stable values.
type Prefix interface {
	// Loaded reports whether the prefix transport is currently loaded.
	Loaded() bool
	// ExchangeIssuer runs the issuer-bootstrap exchange over this prefix.
	ExchangeIssuer(ctx context.Context, present client.ClosedTokenPresenter, batch []byte) (client.ClosedIssuanceExchangeResult, error)
}
