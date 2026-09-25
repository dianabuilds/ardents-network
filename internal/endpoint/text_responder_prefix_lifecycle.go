//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// textResponderPrefixLifecycle is the sole owner of the Publisher's live
// Responder prefix and an opening that may replace its absence.
type textResponderPrefixLifecycle struct {
	live    *textResponderPrefixHandle
	opening *textSourceFlight
	set     *textInteriorSet
}

// textResponderPrefixHandle exposes only operations belonging to one exact
// live Responder prefix. Retirement invalidates every retained handle.
type textResponderPrefixHandle struct {
	owner  *textResponderPrefixLifecycle
	prefix atomic.Pointer[route.ClosedSourcePrefix]
	cancel context.CancelFunc
}

type textResponderPrefixRetirement struct {
	prefix  *route.ClosedSourcePrefix
	opening *textSourceFlight
}

// textResponderJoinAcquisition binds one JOIN exchange to the exact Responder
// handle and Source issuer current at admission.
type textResponderJoinAcquisition struct {
	handle atomic.Pointer[textResponderPrefixHandle]
	issuer *textSourceHandle
}

func (lifecycle *textResponderPrefixLifecycle) currentLocked() *textResponderPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.live
}

func (lifecycle *textResponderPrefixLifecycle) acquireOpenedLocked(prefix *route.ClosedSourcePrefix) *textResponderPrefixHandle {
	handle := lifecycle.currentLocked()
	if handle == nil || prefix == nil || handle.prefix.Load() != prefix {
		return nil
	}
	return handle
}

func (lifecycle *textResponderPrefixLifecycle) acquireJoinLocked(issuer *textSourceHandle) *textResponderJoinAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &textResponderJoinAcquisition{issuer: issuer}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

func (lifecycle *textResponderPrefixLifecycle) openingAvailableLocked() bool {
	return lifecycle != nil && lifecycle.live == nil && lifecycle.opening == nil
}

func (lifecycle *textResponderPrefixLifecycle) membersSlotLocked() **textInteriorSet {
	return &lifecycle.set
}

func (lifecycle *textResponderPrefixLifecycle) reserveOpeningLocked(flight *textSourceFlight) bool {
	if lifecycle == nil || flight == nil || lifecycle.live != nil || lifecycle.opening != nil {
		return false
	}
	lifecycle.opening = flight
	return true
}

func (lifecycle *textResponderPrefixLifecycle) openingCurrentLocked(flight *textSourceFlight) bool {
	return lifecycle != nil && lifecycle.opening == flight
}

func (lifecycle *textResponderPrefixLifecycle) finishOpeningLocked(flight *textSourceFlight,
	prefix *route.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) bool {
	if lifecycle == nil || lifecycle.opening != flight {
		return false
	}
	lifecycle.opening = nil
	if !publish || prefix == nil {
		return true
	}
	handle := &textResponderPrefixHandle{owner: lifecycle, cancel: cancel}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return true
}

func (lifecycle *textResponderPrefixLifecycle) retireIdleLocked() error {
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

func (lifecycle *textResponderPrefixLifecycle) stopLocked() *textResponderPrefixRetirement {
	retirement := &textResponderPrefixRetirement{}
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

func (retirement *textResponderPrefixRetirement) joinOpening() {
	if retirement != nil && retirement.opening != nil {
		<-retirement.opening.done
		retirement.opening = nil
	}
}

func (retirement *textResponderPrefixRetirement) closePrefix() error {
	if retirement == nil || retirement.prefix == nil {
		return nil
	}
	prefix := retirement.prefix
	retirement.prefix = nil
	return prefix.Close()
}

func (handle *textResponderPrefixHandle) currentLocked(lifecycle *textResponderPrefixLifecycle) bool {
	return handle != nil && lifecycle != nil && lifecycle.live == handle && handle.owner == lifecycle && handle.prefix.Load() != nil
}

func (handle *textResponderPrefixHandle) routePrefix() (*route.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New("text Responder prefix unavailable")
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New("text Responder prefix unavailable")
	}
	return prefix, nil
}

func (handle *textResponderPrefixHandle) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return prefix.DataJoinRecipient()
}

func (handle *textResponderPrefixHandle) join(ctx context.Context, present route.ClosedTokenPresenter,
	intent route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.Join(ctx, present, intent)
}

func (handle *textResponderPrefixHandle) replenish(ctx context.Context, present route.ClosedTokenPresenter) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

func (handle *textResponderPrefixHandle) retired() bool {
	prefix, err := handle.routePrefix()
	if err != nil {
		return true
	}
	select {
	case <-prefix.Done():
		return true
	default:
		return false
	}
}

func (acquisition *textResponderJoinAcquisition) release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
		acquisition.issuer = nil
	}
}

func (acquisition *textResponderJoinAcquisition) currentLocked(owner *textContext) bool {
	if acquisition == nil || owner == nil || owner.surface != broker.Administration {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.currentLocked(&owner.responder) && acquisition.issuer != nil && acquisition.issuer.currentLocked(owner)
}

func (acquisition *textResponderJoinAcquisition) issuancePrefixLocked(owner *textContext) (*textSourceHandle, bool) {
	current := acquisition.currentLocked(owner)
	if acquisition == nil {
		return nil, false
	}
	return acquisition.issuer, current
}

func (acquisition *textResponderJoinAcquisition) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	if acquisition == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Responder JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Responder JOIN acquisition unavailable")
	}
	return handle.dataJoinRecipient()
}

func (acquisition *textResponderJoinAcquisition) join(ctx context.Context, present route.ClosedTokenPresenter,
	intent route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	if acquisition == nil {
		return nil, errors.New("text Responder JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return nil, errors.New("text Responder JOIN acquisition unavailable")
	}
	return handle.join(ctx, present, intent)
}
