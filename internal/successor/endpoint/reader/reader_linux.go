//go:build linux

package reader

import (
	"context"
	"errors"
	"sync"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
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
	route     *funding.Parents
}

func (owner *Reader) Open(ctx context.Context, request application.Request) (_ application.Stream, result error) {
	if ctx == nil || owner == nil || owner.local == nil || owner.holder == nil || owner.check == nil || !owner.authority.Available() || request.Destination != application.TargetLink {
		return nil, application.Refuse(application.Outcome{Class: application.ServiceUnavailable})
	}
	if err := errors.Join(ctx.Err(), owner.check()); err != nil {
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
	worker, err := owner.local.LaunchReader(ctx, owner.principal)
	if err != nil {
		return nil, err
	}
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return nil, errors.Join(err, worker.Close())
	}
	authority := owner.authority
	originalCurrent := authority.Current
	current := func() (network.RuntimeView, error) {
		// The installed authority reobserves the original Startup before and
		// after CurrentRuntime. Retain this operation's rights around that
		// observation without repeating the same full native file/manager I/O.
		if err := errors.Join(ctx.Err(), operation.CheckReader()); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := originalCurrent()
		return view, errors.Join(err, ctx.Err(), operation.CheckReader())
	}
	authority = funding.Observe(current, authority.Witness, authority.Presentation)
	var service *connection.Stream
	var closeSource func() error
	var retireOnce sync.Once
	var retired error
	retire := func() error {
		retireOnce.Do(func() {
			if closeSource != nil {
				retired = closeSource()
			} else if service != nil {
				retired = service.Close()
				operation.RetainCleanup(service.JoinedCleanup())
			}
			operation.Close()
			retired = errors.Join(retired, worker.Close())
		})
		return retired
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, retire())
		}
	}()
	contract, err := operation.BindStreamContract(512, uint64(textdocument.MaximumBytes+13))
	if err != nil {
		return nil, err
	}
	var source *connection.InitiatorSource
	source, closeSource, err = OpenSource(operation, plan, authority, owner.holder, request.Value, owner.route)
	if err != nil {
		return nil, err
	}
	service, err = source.OpenStream(contract)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), owner.check(), operation.CheckReader()); err != nil {
		return nil, err
	}
	complete := func() ([]byte, error) {
		transferred, response, err := operation.DeliverReaderStream(contract, service)
		if !transferred {
			_ = service.Close()
			operation.RetainCleanup(service.JoinedCleanup())
		}
		if err != nil || response == nil {
			return nil, errors.Join(err, errors.New("original Reader result is unavailable"), retire())
		}
		defer response.Close()
		if err := errors.Join(retire(), ctx.Err(), owner.check()); err != nil {
			return nil, err
		}
		return response.TakeBytes()
	}
	// The local parser admits the fixed request before the sole confined
	// worker transfer. The caller receives no raw Service transport.
	return newResultStream(ctx, owner.check, complete, retire), nil
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
	Parents   *funding.Parents
}

// New binds the participant. Every Open independently rechecks its original
// admission and launches one qualified Reader; construction is not acceptance.
func New(c Config) *Reader {
	return &Reader{local: c.Local, principal: c.Principal, plan: c.Plan, authority: c.Authority, holder: c.Holder, check: c.Check, route: c.Parents}
}
