//go:build linux

// Package source owns the live Source route opening of one duty context: the
// published read-only Handle, its in-progress replacement slot, and the exact
// acquisitions that bind one JOIN or resolution exchange to the handle current
// at admission. Lifecycle owns its operation reservation across prefix
// replacement. Its opening state is valid only under the owning dutyContext
// mutex; the retained Interior Set and role selection stay with the endpoint
// owner.
package source

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// FlightRef is the seam to the endpoint's operation flight, which fuses the
// Source opening lifetime with the duty context and is shared with publication
// withdrawal and role prefix openings. Identity comparisons between the
// retained reference and a caller's concrete flight stay exact through
// interface boxing.
type FlightRef interface {
	CancelFlight()
	JoinFlight()
}

// Lifecycle owns the live Source opening, its in-progress replacement, and
// the operation reservation across replacements. The zero value is ready.
type Lifecycle struct {
	live      *Handle
	opening   FlightRef
	operation operationGate
}

// Retirement is one concrete snapshot of what the Lifecycle held at stop
// time. It contains no admission path.
type Retirement struct {
	prefix  *client.ClosedSourcePrefix
	opening FlightRef
}

// Handle is a read-only capability for one exact published Source opening.
// It deliberately exposes neither Close nor the underlying prefix; production
// retires routes only through the Lifecycle. The tracked test seams in
// testsupport.go are the sole other path.
type Handle struct {
	owner  *Lifecycle
	prefix atomic.Pointer[client.ClosedSourcePrefix]
	cancel context.CancelFunc
}

// JoinAcquisition binds one JOIN exchange to the exact Source handle current
// at admission. Releasing it cannot expose a replacement handle.
type JoinAcquisition struct {
	handle atomic.Pointer[Handle]
}

// ResolutionAcquisition binds one Descriptor lookup to the exact Source
// handle current at admission. Release is local to this lookup and can never
// expose a replacement handle.
type ResolutionAcquisition struct {
	handle atomic.Pointer[Handle]
}

func (lifecycle *Lifecycle) AcquireJoinLocked() *JoinAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &JoinAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

func (lifecycle *Lifecycle) AcquireResolutionLocked() *ResolutionAcquisition {
	if lifecycle == nil || lifecycle.live == nil || lifecycle.live.prefix.Load() == nil {
		return nil
	}
	acquisition := &ResolutionAcquisition{}
	acquisition.handle.Store(lifecycle.live)
	return acquisition
}

func (acquisition *JoinAcquisition) Release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *JoinAcquisition) CurrentLocked(lifecycle *Lifecycle) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.CurrentLocked(lifecycle)
}

func (acquisition *JoinAcquisition) IssuancePrefixLocked(lifecycle *Lifecycle) (*Handle, bool) {
	if acquisition == nil {
		return nil, false
	}
	handle := acquisition.handle.Load()
	return handle, handle != nil && handle.CurrentLocked(lifecycle)
}

func (acquisition *JoinAcquisition) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	if acquisition == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, 0, time.Time{}, errors.New("text Source JOIN acquisition unavailable")
	}
	return handle.DataJoinRecipient()
}

func (acquisition *JoinAcquisition) Join(ctx context.Context, present client.ClosedTokenPresenter,
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

func (acquisition *ResolutionAcquisition) Release() {
	if acquisition != nil {
		acquisition.handle.Store(nil)
	}
}

func (acquisition *ResolutionAcquisition) CurrentLocked(lifecycle *Lifecycle) bool {
	if acquisition == nil {
		return false
	}
	handle := acquisition.handle.Load()
	return handle != nil && handle.CurrentLocked(lifecycle)
}

func (acquisition *ResolutionAcquisition) ResolutionRecipient() ([32]byte, error) {
	if acquisition == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	handle := acquisition.handle.Load()
	if handle == nil {
		return [32]byte{}, errors.New("text Source resolution acquisition unavailable")
	}
	return handle.resolutionRecipient()
}

func (acquisition *ResolutionAcquisition) ExchangeDescriptor(ctx context.Context,
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

// CurrentLocked returns the live Source handle. Like the Introduction and
// Responder owners, the Source lifecycle is read through its own owner; the
// Context keeps no pass-through for it.
func (lifecycle *Lifecycle) CurrentLocked() *Handle {
	if lifecycle == nil {
		return nil
	}
	return lifecycle.live
}

func (lifecycle *Lifecycle) OpeningInProgressLocked() bool {
	return lifecycle != nil && lifecycle.opening != nil
}

func (lifecycle *Lifecycle) ReserveOpeningLocked(operation FlightRef) bool {
	if lifecycle == nil || operation == nil || lifecycle.live != nil || lifecycle.opening != nil {
		return false
	}
	lifecycle.opening = operation
	return true
}

func (lifecycle *Lifecycle) OpeningAdmittedLocked(operation FlightRef) bool {
	if lifecycle == nil {
		return false
	}
	if operation == nil {
		return lifecycle.opening == nil
	}
	return lifecycle.opening == operation
}

func (lifecycle *Lifecycle) FinishOpeningLocked(operation FlightRef,
	prefix *client.ClosedSourcePrefix, cancel context.CancelFunc, publish bool) (*Handle, bool) {
	if lifecycle == nil || lifecycle.opening != operation {
		return nil, false
	}
	lifecycle.opening = nil
	if !publish {
		return nil, true
	}
	handle := &Handle{owner: lifecycle, cancel: cancel}
	handle.prefix.Store(prefix)
	lifecycle.live = handle
	return handle, true
}

// CurrentLocked verifies handle identity against the owning lifecycle alone;
// the handle never reaches back into the Context.
func (handle *Handle) CurrentLocked(lifecycle *Lifecycle) bool {
	return handle != nil && lifecycle != nil && handle.prefix.Load() != nil && handle.owner == lifecycle && lifecycle.live == handle
}

func (handle *Handle) routePrefix() (*client.ClosedSourcePrefix, error) {
	if handle == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	prefix := handle.prefix.Load()
	if prefix == nil {
		return nil, errors.New("text Source handle unavailable")
	}
	return prefix, nil
}

func (handle *Handle) resolutionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.ResolutionRecipient()
}

func (handle *Handle) exchangeDescriptor(ctx context.Context, present client.ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, nil, err
	}
	return prefix.ExchangeDescriptor(ctx, present, target, descriptor)
}

func (handle *Handle) SubmissionRecipient() ([32]byte, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, err
	}
	return prefix.SubmissionRecipient()
}

func (handle *Handle) SubmitIntroduction(ctx context.Context, present client.ClosedTokenPresenter, operation []byte) (uint8, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return 0, err
	}
	return prefix.SubmitIntroduction(ctx, present, operation)
}

func (handle *Handle) DataJoinRecipient() ([32]byte, uint64, time.Time, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return [32]byte{}, 0, time.Time{}, err
	}
	return prefix.DataJoinRecipient()
}

func (handle *Handle) join(ctx context.Context, present client.ClosedTokenPresenter, intent client.ClosedJoinIntent) (*client.ClosedJoinedStream, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return nil, err
	}
	return prefix.Join(ctx, present, intent)
}

// Loaded and ExchangeIssuer are the Source side of the token owner's
// Prefix seam: batch identity binding plus the issuer-bootstrap transport.
func (handle *Handle) Loaded() bool {
	return handle != nil && handle.prefix.Load() != nil
}

func (handle *Handle) ExchangeIssuer(ctx context.Context, present client.ClosedTokenPresenter, batch []byte) (client.ClosedIssuanceExchangeResult, error) {
	prefix, err := handle.routePrefix()
	if err != nil {
		return client.ClosedIssuanceExchangeResult{}, err
	}
	return prefix.ExchangeIssuer(ctx, present, batch)
}

func (handle *Handle) Replenish(ctx context.Context, present client.ClosedTokenPresenter) error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Replenish(ctx, present)
}

func (lifecycle *Lifecycle) RetireIdleLocked() error {
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

func (lifecycle *Lifecycle) detachLocked() (*client.ClosedSourcePrefix, FlightRef) {
	handle, opening := lifecycle.live, lifecycle.opening
	lifecycle.live, lifecycle.opening = nil, nil
	var prefix *client.ClosedSourcePrefix
	if handle != nil {
		handle.cancel()
		prefix = handle.prefix.Swap(nil)
	}
	if opening != nil {
		opening.CancelFlight()
	}
	return prefix, opening
}

func (lifecycle *Lifecycle) StopLocked() *Retirement {
	prefix, opening := lifecycle.detachLocked()
	return &Retirement{prefix: prefix, opening: opening}
}

func (retirement *Retirement) JoinOpening() {
	if retirement == nil {
		return
	}
	if retirement.opening != nil {
		retirement.opening.JoinFlight()
		retirement.opening = nil
	}
}

func (retirement *Retirement) ClosePrefix() error {
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
