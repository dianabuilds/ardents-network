//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// textResponderPrefixLifecycle is the sole owner of the Publisher's live
// Responder prefix and an opening that may replace its absence. The shared
// role machinery (opening slot, member set, idle retirement, stop) lives in
// rolePrefixCore; this owner adds only the Responder handle type, its
// Route operations and the JOIN acquisition.
type textResponderPrefixLifecycle struct {
	rolePrefixCore
}

// textResponderPrefixHandle exposes only operations belonging to one exact
// live Responder prefix. Retirement invalidates every retained handle.
type textResponderPrefixHandle struct {
	rolePrefixHandleCore
}

// textResponderJoinAcquisition binds one JOIN exchange to the exact Responder
// handle and Source issuer current at admission.
type textResponderJoinAcquisition struct {
	handle atomic.Pointer[textResponderPrefixHandle]
	issuer *sourceHandle
}

func (lifecycle *textResponderPrefixLifecycle) currentLocked() *textResponderPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.currentLiveLocked().(*textResponderPrefixHandle)
	return handle
}

func (lifecycle *textResponderPrefixLifecycle) acquireOpenedLocked(prefix *client.ClosedSourcePrefix) *textResponderPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.acquireOpenedCoreLocked(prefix).(*textResponderPrefixHandle)
	return handle
}

func (lifecycle *textResponderPrefixLifecycle) acquireJoinLocked(issuer *sourceHandle) *textResponderJoinAcquisition {
	live := lifecycle.currentLocked()
	if live == nil || live.prefix.Load() == nil {
		return nil
	}
	acquisition := &textResponderJoinAcquisition{issuer: issuer}
	acquisition.handle.Store(live)
	return acquisition
}

func (lifecycle *textResponderPrefixLifecycle) finishOpeningLocked(flight *textOperationFlight,
	prefix *client.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) bool {
	if lifecycle == nil || lifecycle.opening != flight {
		return false
	}
	lifecycle.opening = nil
	if !publish || prefix == nil {
		return true
	}
	handle := &textResponderPrefixHandle{rolePrefixHandleCore: rolePrefixHandleCore{owner: &lifecycle.rolePrefixCore, cancel: cancel}}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return true
}

func (handle *textResponderPrefixHandle) currentLocked(lifecycle *textResponderPrefixLifecycle) bool {
	return handle != nil && lifecycle != nil && handle.rolePrefixHandleCore.currentCoreLocked(&lifecycle.rolePrefixCore)
}

func (handle *textResponderPrefixHandle) routePrefix() (*client.ClosedSourcePrefix, error) {
	return handle.routeCorePrefix("text Responder prefix unavailable")
}

func (handle *textResponderPrefixHandle) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return prefix.DataJoinRecipient()
}

func (handle *textResponderPrefixHandle) join(ctx context.Context, present client.ClosedTokenPresenter,
	intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.Join(ctx, present, intent)
}

func (handle *textResponderPrefixHandle) replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	return handle.replenishCore(ctx, present, "text Responder prefix unavailable")
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
	return handle != nil && handle.currentLocked(&owner.responder) && acquisition.issuer != nil && acquisition.issuer.currentLocked(&owner.source)
}

func (acquisition *textResponderJoinAcquisition) issuancePrefixLocked(owner *textContext) (*sourceHandle, bool) {
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

func (acquisition *textResponderJoinAcquisition) join(ctx context.Context, present client.ClosedTokenPresenter,
	intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	if acquisition == nil {
		return nil, errors.New("text Responder JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return nil, errors.New("text Responder JOIN acquisition unavailable")
	}
	return handle.join(ctx, present, intent)
}
