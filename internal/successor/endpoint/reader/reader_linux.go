//go:build linux

package reader

import (
	"context"
	"errors"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// Reader uses the Endpoint's original qualified local runtime and
// already imported Stock. The retained startup/State observer belongs to the
// constructing installed participant; it is never supplied by an Application.
type Reader struct {
	local     *executionruntime.Owner
	principal [32]byte
	plan      funding.Plan
	authority funding.Authority
	holder    *stock.Owner
	check     func() error
	boundary  func() error
	route     *funding.Parents
}

func (owner *Reader) Open(ctx context.Context, request application.Request) (_ application.Stream, result error) {
	if ctx == nil || owner == nil || owner.local == nil || owner.holder == nil || owner.check == nil || !owner.authority.Available() || request.Destination != application.TargetLink {
		return nil, application.Refuse(application.Outcome{Class: application.ServiceUnavailable})
	}
	// Operation admission boundary: full-content installed observation. Inner
	// effect brackets below use the identity-only check.
	if err := errors.Join(ctx.Err(), owner.boundary()); err != nil {
		return nil, err
	}
	view, err := owner.authority.Current()
	if err != nil {
		return nil, err
	}
	plan := owner.plan
	plan.Deadline = time.Now().Add(admission.ForwardClass.Lifetime()).UTC().Truncate(time.Second)
	if end := view.Profile().NotAfter; end.Before(plan.Deadline) {
		plan.Deadline = end
	}
	if end, bounded := ctx.Deadline(); bounded && end.Before(plan.Deadline) {
		plan.Deadline = end.UTC().Truncate(time.Second)
	}
	read, err := Begin(ctx, owner.local, owner.principal)
	if err != nil {
		return nil, err
	}
	authority := owner.authority
	originalCurrent := authority.Current
	current := func() (network.RuntimeView, error) {
		// The installed authority reobserves the original Startup before and
		// after CurrentRuntime. Retain this operation's rights around that
		// observation without repeating the same full native file/manager I/O.
		if err := errors.Join(ctx.Err(), read.Check()); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := originalCurrent()
		return view, errors.Join(err, ctx.Err(), read.Check())
	}
	authority = funding.Observe(current, authority.Witness, authority.Presentation)
	pending, err := read.prepare(plan, authority, owner.holder, request.Value, owner.route)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), owner.check(), read.Check()); err != nil {
		return nil, errors.Join(err, pending.close())
	}
	// The fixed local request precedes the same sole worker transfer used by
	// the console adapter. The caller receives no raw Service transport.
	return newResultStream(ctx, owner.check, func() ([]byte, error) {
		return pending.complete(owner.check)
	}, pending.close), nil
}

// Config binds one original participant's retained owners. Reader borrows them;
// the participant closes Stock, Network, Execution and Parents after requests join.
type Config struct {
	Local     *executionruntime.Owner
	Principal [32]byte
	Plan      funding.Plan
	Authority funding.Authority
	Holder    *stock.Owner
	Check     func() error
	// Boundary is the full-content installed observation at each read operation's
	// admission entry; Check is the identity-only inner bracket. When Boundary is
	// nil it falls back to Check (the caller then supplies one granularity).
	Boundary func() error
	Parents  *funding.Parents
}

// New binds the participant. Every Open independently rechecks its original
// admission and launches one qualified Reader; construction is not acceptance.
func New(c Config) *Reader {
	boundary := c.Boundary
	if boundary == nil {
		boundary = c.Check
	}
	return &Reader{local: c.Local, principal: c.Principal, plan: c.Plan, authority: c.Authority, holder: c.Holder, check: c.Check, boundary: boundary, route: c.Parents}
}
