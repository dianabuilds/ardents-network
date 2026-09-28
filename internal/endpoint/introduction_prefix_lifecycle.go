//go:build linux

package endpoint

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// introductionPrefixLifecycle is the sole owner of the Publisher's live
// Introduction prefix and an opening that may replace its absence. The shared
// role machinery (opening slot, member set, idle retirement, stop) lives in
// rolePrefixCore; this owner adds only the Introduction handle type and
// its Route operations.
type introductionPrefixLifecycle struct {
	rolePrefixCore
}

// introductionPrefixHandle exposes only operations belonging to the exact
// live Introduction prefix. Retirement invalidates every retained handle.
type introductionPrefixHandle struct {
	rolePrefixHandleCore
}

func (lifecycle *introductionPrefixLifecycle) currentLocked() *introductionPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.currentLiveLocked().(*introductionPrefixHandle)
	return handle
}

func (lifecycle *introductionPrefixLifecycle) acquireOpenedLocked(prefix *client.ClosedSourcePrefix) *introductionPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.acquireOpenedCoreLocked(prefix).(*introductionPrefixHandle)
	return handle
}

func (lifecycle *introductionPrefixLifecycle) finishOpeningLocked(flight *textOperationFlight,
	prefix *client.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) bool {
	if lifecycle == nil || lifecycle.opening != flight {
		return false
	}
	lifecycle.opening = nil
	if !publish || prefix == nil {
		return true
	}
	handle := &introductionPrefixHandle{rolePrefixHandleCore: rolePrefixHandleCore{owner: &lifecycle.rolePrefixCore, cancel: cancel}}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return true
}

func (handle *introductionPrefixHandle) currentLocked(lifecycle *introductionPrefixLifecycle) bool {
	return handle != nil && lifecycle != nil && handle.rolePrefixHandleCore.currentCoreLocked(&lifecycle.rolePrefixCore)
}

func (handle *introductionPrefixHandle) routePrefix() (*client.ClosedSourcePrefix, error) {
	return handle.routeCorePrefix("text Introduction prefix unavailable")
}

func (handle *introductionPrefixHandle) introductionRecipient() ([32]byte, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, time.Time{}, err
	}
	return prefix.IntroductionRecipient()
}

func (handle *introductionPrefixHandle) register(ctx context.Context, present client.ClosedTokenPresenter,
	request terminal.RegistrationRequest) (*client.ClosedIntroductionRegistration, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.RegisterIntroduction(ctx, present, request)
}

func (handle *introductionPrefixHandle) replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	return handle.replenishCore(ctx, present, "text Introduction prefix unavailable")
}
