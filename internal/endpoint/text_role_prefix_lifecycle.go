//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// textRolePrefixLive is the installed handle of one Publisher role prefix.
// The Introduction and Responder handles each embed textRolePrefixHandleCore
// and add only their role-specific Route operations. The shared machinery
// never mixes the roles: a core's live slot is written only by the owning
// role's finishOpeningLocked.
type textRolePrefixLive interface {
	rolePrefixCore() *textRolePrefixHandleCore
}

// textRolePrefixHandleCore is the identity and cancellation core shared by
// both Publisher role prefix handles. owner binds the handle to the exact
// role lifecycle core that installed it; retirement and replacement
// invalidate every retained handle by clearing prefix and live.
type textRolePrefixHandleCore struct {
	owner  *textRolePrefixCore
	prefix atomic.Pointer[client.ClosedSourcePrefix]
	cancel context.CancelFunc
}

func (handle *textRolePrefixHandleCore) rolePrefixCore() *textRolePrefixHandleCore {
	return handle
}

// currentCoreLocked reports whether the handle is the exact live handle of
// the role lifecycle core and still holds its Route prefix.
func (handle *textRolePrefixHandleCore) currentCoreLocked(core *textRolePrefixCore) bool {
	return handle != nil && core != nil && handle.owner == core && core.liveCoreLocked() == handle && handle.prefix.Load() != nil
}

func (handle *textRolePrefixHandleCore) routeCorePrefix(unavailable string) (*client.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New(unavailable)
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New(unavailable)
	}
	return prefix, nil
}

func (handle *textRolePrefixHandleCore) replenishCore(ctx context.Context, present client.ClosedTokenPresenter, unavailable string) error {
	prefix, err := handle.routeCorePrefix(unavailable)
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

// textRolePrefixCore is the single opening/members machinery shared by the
// Publisher Introduction and Responder prefix lifecycles: one live handle
// slot, one opening flight slot, one interior member set, and the identical
// idle-retirement and stop transitions. The Source keeps its own lifecycle:
// its opening admission, operations gate and acquisitions diverge.
type textRolePrefixCore struct {
	live    textRolePrefixLive
	opening *textOperationFlight
	set     *textInteriorSet
}

func (core *textRolePrefixCore) liveCoreLocked() *textRolePrefixHandleCore {
	if core == nil || core.live == nil {
		return nil
	}
	return core.live.rolePrefixCore()
}

func (core *textRolePrefixCore) currentLiveLocked() textRolePrefixLive {
	if core == nil {
		return nil
	}
	return core.live
}

func (core *textRolePrefixCore) acquireOpenedCoreLocked(prefix *client.ClosedSourcePrefix) textRolePrefixLive {
	live := core.currentLiveLocked()
	if live == nil || prefix == nil || live.rolePrefixCore().prefix.Load() != prefix {
		return nil
	}
	return live
}

func (core *textRolePrefixCore) openingInProgressLocked() bool {
	return core != nil && core.opening != nil
}

func (core *textRolePrefixCore) openingAvailableLocked() bool {
	return core != nil && core.live == nil && core.opening == nil
}

func (core *textRolePrefixCore) membersSlotLocked() **textInteriorSet {
	return &core.set
}

func (core *textRolePrefixCore) reserveOpeningLocked(flight *textOperationFlight) bool {
	if core == nil || flight == nil || core.live != nil || core.opening != nil {
		return false
	}
	core.opening = flight
	return true
}

func (core *textRolePrefixCore) openingCurrentLocked(flight *textOperationFlight) bool {
	return core != nil && core.opening == flight
}

// retireIdleLocked closes the live prefix only after Route has ended it.
// The close runs under textContext.mu like every idle retirement.
func (core *textRolePrefixCore) retireIdleLocked() error {
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
func (core *textRolePrefixCore) stopLocked() *textRolePrefixRetirement {
	retirement := &textRolePrefixRetirement{}
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

// textRolePrefixRetirement carries the stopped prefix and opening of one
// Publisher role out of the locked stop phase; joining the opening and
// closing the prefix run without textContext.mu.
type textRolePrefixRetirement struct {
	prefix  *client.ClosedSourcePrefix
	opening *textOperationFlight
}

func (retirement *textRolePrefixRetirement) joinOpening() {
	if retirement == nil {
		return
	}
	retirement.opening.join()
	retirement.opening = nil
}

func (retirement *textRolePrefixRetirement) closePrefix() error {
	if retirement == nil || retirement.prefix == nil {
		return nil
	}
	prefix := retirement.prefix
	retirement.prefix = nil
	return prefix.Close()
}
