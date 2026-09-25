//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// textSourceLifecycle is the only owner of the live Source opening and its
// in-progress replacement. The zero value is ready for use under textContext.mu.
type textSourceLifecycle struct {
	live       *textSourceHandle
	opening    *textPrefixOpeningOperation
	operations textSourceOperationGate
}

type textSourceRetirement struct {
	prefix  *route.ClosedSourcePrefix
	opening *textPrefixOpeningOperation
}

// textSourceHandle is a read-only capability for one exact published Source
// opening. It deliberately exposes neither Close nor the underlying prefix.
type textSourceHandle struct {
	owner  *textSourceLifecycle
	prefix atomic.Pointer[route.ClosedSourcePrefix]
	cancel context.CancelFunc
}

// textSourceJoinAcquisition binds one JOIN exchange to the exact Source handle
// current at admission. Releasing it cannot expose a replacement handle.
type textSourceJoinAcquisition struct {
	handle atomic.Pointer[textSourceHandle]
}

func (lifecycle *textSourceLifecycle) acquireJoinLocked() *textSourceJoinAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &textSourceJoinAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

// textSourceResolutionAcquisition binds one Descriptor lookup to the exact
// Source handle current at admission. Release is local to this lookup and can
// never expose a replacement handle.
type textSourceResolutionAcquisition struct {
	handle atomic.Pointer[textSourceHandle]
}

func (lifecycle *textSourceLifecycle) acquireResolutionLocked() *textSourceResolutionAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &textSourceResolutionAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

func (acquisition *textSourceJoinAcquisition) release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *textSourceJoinAcquisition) currentLocked(owner *textContext) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.currentLocked(owner)
}

func (acquisition *textSourceJoinAcquisition) issuancePrefixLocked(owner *textContext) (*textSourceHandle, bool) {
	if acquisition == nil {
		return nil, false
	}
	handle := acquisition.handle.Load()
	return handle, handle != nil && handle.currentLocked(owner)
}

func (acquisition *textSourceJoinAcquisition) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	if acquisition == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	return handle.dataJoinRecipient()
}

func (acquisition *textSourceJoinAcquisition) join(ctx context.Context, present route.ClosedTokenPresenter,
	intent route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	if acquisition == nil {
		return nil, errors.New("text Source JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return nil, errors.New("text Source JOIN acquisition unavailable")
	}
	return handle.join(ctx, present, intent)
}

func (acquisition *textSourceResolutionAcquisition) release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *textSourceResolutionAcquisition) currentLocked(owner *textContext) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.currentLocked(owner)
}

func (acquisition *textSourceResolutionAcquisition) resolutionRecipient() ([32]byte, error) {
	if acquisition == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	return handle.resolutionRecipient()
}

func (acquisition *textSourceResolutionAcquisition) exchangeDescriptor(ctx context.Context,
	present route.ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	if acquisition == nil {
		return 0, nil, errors.New("text Source resolution acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return 0, nil, errors.New("text Source resolution acquisition unavailable")
	}
	return handle.exchangeDescriptor(ctx, present, target, descriptor)
}

func (owner *textContext) currentTextSourceLocked() *textSourceHandle {
	if owner == nil {
		return nil
	}
	return owner.source.live
}

func (lifecycle *textSourceLifecycle) openingInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

func (lifecycle *textSourceLifecycle) reserveOpeningLocked(operation *textPrefixOpeningOperation) bool {
	if lifecycle == nil || operation == nil || lifecycle.live != nil || lifecycle.opening != nil {
		return false
	}
	lifecycle.opening = operation
	return true
}

func (lifecycle *textSourceLifecycle) openingAdmittedLocked(operation *textPrefixOpeningOperation) bool {
	if lifecycle == nil {
		return false
	}
	if operation == nil {
		return lifecycle.opening == nil
	}
	return lifecycle.opening == operation
}

func (lifecycle *textSourceLifecycle) finishOpeningLocked(operation *textPrefixOpeningOperation,
	prefix *route.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) (*textSourceHandle, bool) {
	if lifecycle == nil || lifecycle.opening != operation {
		return nil, false
	}
	lifecycle.opening = nil
	if !publish {
		return nil, true
	}
	handle := &textSourceHandle{owner: lifecycle, cancel: cancel}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return handle, true
}

func (handle *textSourceHandle) currentLocked(owner *textContext) bool {
	return handle != nil && owner != nil && handle.prefix.Load() != nil && handle.owner == &owner.source && owner.source.live == handle
}

func (handle *textSourceHandle) routePrefix() (*route.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	return prefix, nil
}

func (handle *textSourceHandle) resolutionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.ResolutionRecipient()
}

func (handle *textSourceHandle) exchangeDescriptor(ctx context.Context, present route.ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, nil, err
	}
	return prefix.ExchangeDescriptor(ctx, present, target, descriptor)
}

func (handle *textSourceHandle) submissionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.SubmissionRecipient()
}

func (handle *textSourceHandle) submitIntroduction(ctx context.Context, present route.ClosedTokenPresenter, operation []byte) (uint8, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, err
	}
	return prefix.SubmitIntroduction(ctx, present, operation)
}

func (handle *textSourceHandle) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return prefix.DataJoinRecipient()
}

func (handle *textSourceHandle) join(ctx context.Context, present route.ClosedTokenPresenter, intent route.ClosedJoinIntent) (*route.ClosedJoinedStream, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.Join(ctx, present, intent)
}

func (handle *textSourceHandle) exchangeIssuer(ctx context.Context, present route.ClosedTokenPresenter, batch []byte) (route.ClosedIssuanceExchangeResult, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return route.ClosedIssuanceExchangeResult{}, err
	}
	return prefix.ExchangeIssuer(ctx, present, batch)
}

func (handle *textSourceHandle) replenish(ctx context.Context, present route.ClosedTokenPresenter) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

func (lifecycle *textSourceLifecycle) retireIdleLocked() error {
	handle := lifecycle.live
	if handle == nil {
		return nil
	}
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

func (lifecycle *textSourceLifecycle) detachLocked() (*route.ClosedSourcePrefix, *textPrefixOpeningOperation) {
	handle, opening := lifecycle.live, lifecycle.opening
	lifecycle.live, lifecycle.opening = nil, nil
	var prefix *route.ClosedSourcePrefix
	if handle != nil {
		handle.cancel()
		prefix = handle.prefix.Swap(nil)
	}
	if opening != nil {
		opening.cancel()
	}
	return prefix, opening
}

func (lifecycle *textSourceLifecycle) stopLocked() *textSourceRetirement {
	prefix, opening := lifecycle.detachLocked()
	return &textSourceRetirement{prefix: prefix, opening: opening}
}

func (retirement *textSourceRetirement) joinOpening() {
	if retirement == nil {
		return
	}
	if retirement.opening != nil {
		retirement.opening.join()
		retirement.opening = nil
	}
}

func (retirement *textSourceRetirement) closePrefix() error {
	if retirement == nil {
		return nil
	}
	if retirement.prefix == nil {
		return nil
	}
	prefix := retirement.prefix
	retirement.prefix = nil
	return prefix.Close()
}
