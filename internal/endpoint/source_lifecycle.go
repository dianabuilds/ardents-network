//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// sourceLifecycle is the only owner of the live Source opening and its
// in-progress replacement. The zero value is ready for use under textContext.mu.
type sourceLifecycle struct {
	live       *sourceHandle
	opening    *textOperationFlight
	operations sourceOperationGate
	set        *textInteriorSet
}

// The retained Interior Set survives prefix replacement and belongs to this
// context's Source lifecycle, not to an individual Route opening.
func (lifecycle *sourceLifecycle) membersSlotLocked() **textInteriorSet {
	return &lifecycle.set
}

func (lifecycle *sourceLifecycle) hasMembersLocked() bool {
	return lifecycle != nil && lifecycle.set != nil
}

type sourceRetirement struct {
	prefix  *client.ClosedSourcePrefix
	opening *textOperationFlight
}

// sourceHandle is a read-only capability for one exact published Source
// opening. It deliberately exposes neither Close nor the underlying prefix.
type sourceHandle struct {
	owner  *sourceLifecycle
	prefix atomic.Pointer[client.ClosedSourcePrefix]
	cancel context.CancelFunc
}

// sourceJoinAcquisition binds one JOIN exchange to the exact Source handle
// current at admission. Releasing it cannot expose a replacement handle.
type sourceJoinAcquisition struct {
	handle atomic.Pointer[sourceHandle]
}

func (lifecycle *sourceLifecycle) acquireJoinLocked() *sourceJoinAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &sourceJoinAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

// sourceResolutionAcquisition binds one Descriptor lookup to the exact
// Source handle current at admission. Release is local to this lookup and can
// never expose a replacement handle.
type sourceResolutionAcquisition struct {
	handle atomic.Pointer[sourceHandle]
}

func (lifecycle *sourceLifecycle) acquireResolutionLocked() *sourceResolutionAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &sourceResolutionAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

func (acquisition *sourceJoinAcquisition) release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *sourceJoinAcquisition) currentLocked(owner *textContext) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.currentLocked(&owner.source)
}

func (acquisition *sourceJoinAcquisition) issuancePrefixLocked(owner *textContext) (*sourceHandle, bool) {
	if acquisition == nil {
		return nil, false
	}
	handle := acquisition.handle.Load()
	return handle, handle != nil && handle.currentLocked(&owner.source)
}

func (acquisition *sourceJoinAcquisition) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	if acquisition == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	return handle.dataJoinRecipient()
}

func (acquisition *sourceJoinAcquisition) join(ctx context.Context, present client.ClosedTokenPresenter,
	intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	if acquisition == nil {
		return nil, errors.New("text Source JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return nil, errors.New("text Source JOIN acquisition unavailable")
	}
	return handle.join(ctx, present, intent)
}

func (acquisition *sourceResolutionAcquisition) release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *sourceResolutionAcquisition) currentLocked(lifecycle *sourceLifecycle) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.currentLocked(lifecycle)
}

func (acquisition *sourceResolutionAcquisition) resolutionRecipient() ([32]byte, error) {
	if acquisition == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	return handle.resolutionRecipient()
}

func (acquisition *sourceResolutionAcquisition) exchangeDescriptor(ctx context.Context,
	present client.ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	if acquisition == nil {
		return 0, nil, errors.New("text Source resolution acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return 0, nil, errors.New("text Source resolution acquisition unavailable")
	}
	return handle.exchangeDescriptor(ctx, present, target, descriptor)
}

// currentLocked returns the live Source handle. Like the Introduction and
// Responder owners, the Source lifecycle is read through its own owner; the
// Context keeps no pass-through for it.
func (lifecycle *sourceLifecycle) currentLocked() *sourceHandle {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.live
}

func (lifecycle *sourceLifecycle) openingInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

func (lifecycle *sourceLifecycle) reserveOpeningLocked(operation *textOperationFlight) bool {
	if lifecycle == nil || operation == nil || lifecycle.live != nil || lifecycle.opening != nil {
		return false
	}
	lifecycle.opening = operation
	return true
}

func (lifecycle *sourceLifecycle) openingAdmittedLocked(operation *textOperationFlight) bool {
	if lifecycle == nil {
		return false
	}
	if operation == nil {
		return lifecycle.opening == nil
	}
	return lifecycle.opening == operation
}

func (lifecycle *sourceLifecycle) finishOpeningLocked(operation *textOperationFlight,
	prefix *client.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) (*sourceHandle, bool) {
	if lifecycle == nil || lifecycle.opening != operation {
		return nil, false
	}
	lifecycle.opening = nil
	if !publish {
		return nil, true
	}
	handle := &sourceHandle{owner: lifecycle, cancel: cancel}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return handle, true
}

// currentLocked verifies handle identity against the owning lifecycle alone;
// the handle never reaches back into the Context.
func (handle *sourceHandle) currentLocked(lifecycle *sourceLifecycle) bool {
	return handle != nil && lifecycle != nil && handle.prefix.Load() != nil && handle.owner == lifecycle && lifecycle.live == handle
}

func (handle *sourceHandle) routePrefix() (*client.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	return prefix, nil
}

func (handle *sourceHandle) resolutionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.ResolutionRecipient()
}

func (handle *sourceHandle) exchangeDescriptor(ctx context.Context, present client.ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, nil, err
	}
	return prefix.ExchangeDescriptor(ctx, present, target, descriptor)
}

func (handle *sourceHandle) submissionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.SubmissionRecipient()
}

func (handle *sourceHandle) submitIntroduction(ctx context.Context, present client.ClosedTokenPresenter, operation []byte) (uint8, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, err
	}
	return prefix.SubmitIntroduction(ctx, present, operation)
}

func (handle *sourceHandle) dataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return prefix.DataJoinRecipient()
}

func (handle *sourceHandle) join(ctx context.Context, present client.ClosedTokenPresenter, intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.Join(ctx, present, intent)
}

func (handle *sourceHandle) exchangeIssuer(ctx context.Context, present client.ClosedTokenPresenter, batch []byte) (client.ClosedIssuanceExchangeResult, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return client.ClosedIssuanceExchangeResult{}, err
	}
	return prefix.ExchangeIssuer(ctx, present, batch)
}

func (handle *sourceHandle) replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

func (lifecycle *sourceLifecycle) retireIdleLocked() error {
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

func (lifecycle *sourceLifecycle) detachLocked() (*client.ClosedSourcePrefix, *textOperationFlight) {
	handle, opening := lifecycle.live, lifecycle.opening
	lifecycle.live, lifecycle.opening = nil, nil
	var prefix *client.ClosedSourcePrefix
	if handle != nil {
		handle.cancel()
		prefix = handle.prefix.Swap(nil)
	}
	if opening != nil {
		opening.cancel()
	}
	return prefix, opening
}

func (lifecycle *sourceLifecycle) stopLocked() *sourceRetirement {
	prefix, opening := lifecycle.detachLocked()
	lifecycle.set = nil
	return &sourceRetirement{prefix: prefix, opening: opening}
}

func (retirement *sourceRetirement) joinOpening() {
	if retirement == nil {
		return
	}
	if retirement.opening != nil {
		retirement.opening.join()
		retirement.opening = nil
	}
}

func (retirement *sourceRetirement) closePrefix() error {
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
