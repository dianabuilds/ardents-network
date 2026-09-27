//go:build linux

package endpoint

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// textIntroductionPrefixLifecycle is the sole owner of the Publisher's live
// Introduction prefix and an opening that may replace its absence. The shared
// role machinery (opening slot, member set, idle retirement, stop) lives in
// textRolePrefixCore; this owner adds only the Introduction handle type and
// its Route operations.
type textIntroductionPrefixLifecycle struct {
	textRolePrefixCore
}

// textIntroductionPrefixHandle exposes only operations belonging to the exact
// live Introduction prefix. Retirement invalidates every retained handle.
type textIntroductionPrefixHandle struct {
	textRolePrefixHandleCore
}

func (lifecycle *textIntroductionPrefixLifecycle) currentLocked() *textIntroductionPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.currentLiveLocked().(*textIntroductionPrefixHandle)
	return handle
}

func (lifecycle *textIntroductionPrefixLifecycle) acquireOpenedLocked(prefix *client.ClosedSourcePrefix) *textIntroductionPrefixHandle {
	if lifecycle == nil {
		return nil
	}
	handle, _ := lifecycle.acquireOpenedCoreLocked(prefix).(*textIntroductionPrefixHandle)
	return handle
}

func (lifecycle *textIntroductionPrefixLifecycle) finishOpeningLocked(flight *textOperationFlight,
	prefix *client.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) bool {
	if lifecycle == nil || lifecycle.opening != flight {
		return false
	}
	lifecycle.opening = nil
	if !publish || prefix == nil {
		return true
	}
	handle := &textIntroductionPrefixHandle{textRolePrefixHandleCore: textRolePrefixHandleCore{owner: &lifecycle.textRolePrefixCore, cancel: cancel}}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return true
}

func (handle *textIntroductionPrefixHandle) currentLocked(lifecycle *textIntroductionPrefixLifecycle) bool {
	return handle != nil && lifecycle != nil && handle.textRolePrefixHandleCore.currentCoreLocked(&lifecycle.textRolePrefixCore)
}

func (handle *textIntroductionPrefixHandle) routePrefix() (*client.ClosedSourcePrefix, error) {
	return handle.routeCorePrefix("text Introduction prefix unavailable")
}

func (handle *textIntroductionPrefixHandle) introductionRecipient() ([32]byte, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, time.Time{}, err
	}
	return prefix.IntroductionRecipient()
}

func (handle *textIntroductionPrefixHandle) register(ctx context.Context, present client.ClosedTokenPresenter,
	request terminal.RegistrationRequest) (*client.ClosedIntroductionRegistration, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.RegisterIntroduction(ctx, present, request)
}

func (handle *textIntroductionPrefixHandle) replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	return handle.replenishCore(ctx, present, "text Introduction prefix unavailable")
}
