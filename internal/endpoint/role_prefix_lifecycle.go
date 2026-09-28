//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// rolePrefixLive is the installed handle of one Publisher role prefix.
// The Introduction and Responder handles each embed rolePrefixHandleCore
// and add only their role-specific Route operations. The shared machinery
// never mixes the roles: a core's live slot is written only by the owning
// role's finishOpeningLocked.
type rolePrefixLive interface {
	rolePrefixCore() *rolePrefixHandleCore
}

// rolePrefixHandleCore is the identity and cancellation core shared by
// both Publisher role prefix handles. owner binds the handle to the exact
// role lifecycle core that installed it; retirement and replacement
// invalidate every retained handle by clearing prefix and live.
type rolePrefixHandleCore struct {
	owner  *rolePrefixCore
	prefix atomic.Pointer[client.ClosedSourcePrefix]
	cancel context.CancelFunc
}

func (handle *rolePrefixHandleCore) rolePrefixCore() *rolePrefixHandleCore {
	return handle
}

// currentCoreLocked reports whether the handle is the exact live handle of
// the role lifecycle core and still holds its Route prefix.
func (handle *rolePrefixHandleCore) currentCoreLocked(core *rolePrefixCore) bool {
	return handle != nil && core != nil && handle.owner == core && core.liveCoreLocked() == handle && handle.prefix.Load() != nil
}

func (handle *rolePrefixHandleCore) routeCorePrefix(unavailable string) (*client.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New(unavailable)
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New(unavailable)
	}
	return prefix, nil
}

func (handle *rolePrefixHandleCore) replenishCore(ctx context.Context, present client.ClosedTokenPresenter, unavailable string) error {
	prefix, err := handle.routeCorePrefix(unavailable)
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

// rolePrefixCore is the single opening/members machinery shared by the
// Publisher Introduction and Responder prefix lifecycles: one live handle
// slot, one opening flight slot, one interior member set, and the identical
// idle-retirement and stop transitions. The Source keeps its own lifecycle:
// its opening admission, operations gate and acquisitions diverge.
type rolePrefixCore struct {
	live    rolePrefixLive
	opening *textOperationFlight
	set     *textInteriorSet
}

func (core *rolePrefixCore) liveCoreLocked() *rolePrefixHandleCore {
	if core == nil || core.live == nil {
		return nil
	}
	return core.live.rolePrefixCore()
}

func (core *rolePrefixCore) currentLiveLocked() rolePrefixLive {
	if core == nil {
		return nil
	}
	return core.live
}

func (core *rolePrefixCore) acquireOpenedCoreLocked(prefix *client.ClosedSourcePrefix) rolePrefixLive {
	live := core.currentLiveLocked()
	if live == nil || prefix == nil || live.rolePrefixCore().prefix.Load() != prefix {
		return nil
	}
	return live
}

func (core *rolePrefixCore) openingInProgressLocked() bool {
	return core != nil && core.opening != nil
}

func (core *rolePrefixCore) openingAvailableLocked() bool {
	return core != nil && core.live == nil && core.opening == nil
}

func (core *rolePrefixCore) membersSlotLocked() **textInteriorSet {
	return &core.set
}

func (core *rolePrefixCore) reserveOpeningLocked(flight *textOperationFlight) bool {
	if core == nil || flight == nil || core.live != nil || core.opening != nil {
		return false
	}
	core.opening = flight
	return true
}

func (core *rolePrefixCore) openingCurrentLocked(flight *textOperationFlight) bool {
	return core != nil && core.opening == flight
}

// retireIdleLocked closes the live prefix only after Route has ended it.
// The close runs under textContext.mu like every idle retirement.
func (core *rolePrefixCore) retireIdleLocked() error {
	if core == nil || core.live == nil {
		return nil
	}
	handle := core.live.rolePrefixCore()
	prefix := handle.prefix.Load()
	if prefix == nil {
		core.live = nil
		return nil
	}
	select {
	case <-prefix.Done():
	default:
		return nil
	}
	core.live = nil
	handle.cancel()
	handle.prefix.Store(nil)
	return prefix.Close()
}

// stopLocked revokes the live handle and the opening flight under
// textContext.mu and hands their terminal cleanup to the retirement carrier.
func (core *rolePrefixCore) stopLocked() *rolePrefixRetirement {
	retirement := &rolePrefixRetirement{}
	if core == nil {
		return retirement
	}
	if core.live != nil {
		handle := core.live.rolePrefixCore()
		handle.cancel()
		retirement.prefix = handle.prefix.Swap(nil)
		core.live = nil
	}
	retirement.opening = core.opening
	core.opening = nil
	retirement.opening.cancel()
	core.set = nil
	return retirement
}

// rolePrefixRetirement carries the stopped prefix and opening of one
// Publisher role out of the locked stop phase; joining the opening and
// closing the prefix run without textContext.mu.
type rolePrefixRetirement struct {
	prefix  *client.ClosedSourcePrefix
	opening *textOperationFlight
}

func (retirement *rolePrefixRetirement) joinOpening() {
	if retirement == nil {
		return
	}
	retirement.opening.join()
	retirement.opening = nil
}

func (retirement *rolePrefixRetirement) closePrefix() error {
	if retirement == nil || retirement.prefix == nil {
		return nil
	}
	prefix := retirement.prefix
	retirement.prefix = nil
	return prefix.Close()
}
