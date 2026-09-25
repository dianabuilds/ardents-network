//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// textIntroductionPrefixLifecycle is the sole owner of the Publisher's live
// Introduction prefix and an opening that may replace its absence.
type textIntroductionPrefixLifecycle struct {
	live    *textIntroductionPrefixHandle
	opening *textSourceFlight
	set     *textInteriorSet
}

// textIntroductionPrefixHandle exposes only operations belonging to the exact
// live Introduction prefix. Retirement invalidates every retained handle.
type textIntroductionPrefixHandle struct {
	owner  *textIntroductionPrefixLifecycle
	prefix atomic.Pointer[route.ClosedSourcePrefix]
	cancel context.CancelFunc
}

type textIntroductionPrefixRetirement struct {
	prefix  *route.ClosedSourcePrefix
	opening *textSourceFlight
}

func (lifecycle *textIntroductionPrefixLifecycle) currentLocked() *textIntroductionPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.live
}

func (lifecycle *textIntroductionPrefixLifecycle) acquireOpenedLocked(prefix *route.ClosedSourcePrefix) *textIntroductionPrefixHandle {
	handle := lifecycle.currentLocked()
	if handle == nil || prefix == nil || handle.prefix.Load() != prefix {
		return nil
	}
	return handle
}

func (lifecycle *textIntroductionPrefixLifecycle) openingInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

func (lifecycle *textIntroductionPrefixLifecycle) openingAvailableLocked() bool {
	return lifecycle != nil && lifecycle.live == nil && lifecycle.opening == nil
}

func (lifecycle *textIntroductionPrefixLifecycle) membersSlotLocked() **textInteriorSet {
	return &lifecycle.set
}

func (lifecycle *textIntroductionPrefixLifecycle) reserveOpeningLocked(flight *textSourceFlight) bool {
	if lifecycle == nil || flight == nil || lifecycle.live != nil || lifecycle.opening != nil {
		return false
	}
	lifecycle.opening = flight
	return true
}

func (lifecycle *textIntroductionPrefixLifecycle) openingCurrentLocked(flight *textSourceFlight) bool {
	return lifecycle != nil && lifecycle.opening == flight
}

func (lifecycle *textIntroductionPrefixLifecycle) finishOpeningLocked(flight *textSourceFlight,
	prefix *route.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) bool {
	if lifecycle == nil || lifecycle.opening != flight {
		return false
	}
	lifecycle.opening = nil
	if !publish || prefix == nil {
		return true
	}
	handle := &textIntroductionPrefixHandle{owner: lifecycle, cancel: cancel}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return true
}

func (lifecycle *textIntroductionPrefixLifecycle) retireIdleLocked() error {
	if lifecycle == nil || lifecycle.live == nil {
		return nil
	}
	handle := lifecycle.live
	prefix := handle.prefix.Load()
	if prefix == nil {
		lifecycle.live = nil
		return nil
	}
	select {
	case <-prefix.Done():
	default:
		return nil
	}
	lifecycle.live = nil
	handle.cancel()
	handle.prefix.Store(nil)
	return prefix.Close()
}

func (lifecycle *textIntroductionPrefixLifecycle) stopLocked() *textIntroductionPrefixRetirement {
	retirement := &textIntroductionPrefixRetirement{}
	if lifecycle == nil {
		return retirement
	}
	if lifecycle.live != nil {
		lifecycle.live.cancel()
		retirement.prefix = lifecycle.live.prefix.Swap(nil)
		lifecycle.live = nil
	}
	retirement.opening = lifecycle.opening
	lifecycle.opening = nil
	if retirement.opening != nil {
		retirement.opening.cancel()
	}
	lifecycle.set = nil
	return retirement
}

func (retirement *textIntroductionPrefixRetirement) joinOpening() {
	if retirement == nil {
		return
	}
	if retirement.opening != nil {
		<-retirement.opening.done
	}
	retirement.opening = nil
}

func (retirement *textIntroductionPrefixRetirement) closePrefix() error {
	if retirement == nil {
		return nil
	}
	if retirement.prefix != nil {
		prefix := retirement.prefix
		retirement.prefix = nil
		return prefix.Close()
	}
	return nil
}

func (handle *textIntroductionPrefixHandle) currentLocked(lifecycle *textIntroductionPrefixLifecycle) bool {
	return handle != nil && lifecycle != nil && lifecycle.live == handle && handle.owner == lifecycle && handle.prefix.Load() != nil
}

func (handle *textIntroductionPrefixHandle) routePrefix() (*route.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New("text Introduction prefix unavailable")
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New("text Introduction prefix unavailable")
	}
	return prefix, nil
}

func (handle *textIntroductionPrefixHandle) introductionRecipient() ([32]byte, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, time.Time{}, err
	}
	return prefix.IntroductionRecipient()
}

func (handle *textIntroductionPrefixHandle) register(ctx context.Context, present route.ClosedTokenPresenter,
	request terminal.RegistrationRequest) (*route.ClosedIntroductionRegistration, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.RegisterIntroduction(ctx, present, request)
}

func (handle *textIntroductionPrefixHandle) replenish(ctx context.Context, present route.ClosedTokenPresenter) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}
