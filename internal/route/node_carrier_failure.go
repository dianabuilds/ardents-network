package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// CarrierFailureClass is the transport-neutral terminal outcome of one
// selected Carrier attempt or authenticated Carrier operation.
type CarrierFailureClass string

const (
	CarrierFailureStale        CarrierFailureClass = "stale"
	CarrierFailureIncompatible CarrierFailureClass = "incompatible"
	CarrierFailureUnauthorized CarrierFailureClass = "unauthorized"
	CarrierFailureCanceled     CarrierFailureClass = "canceled"
	CarrierFailureTimeout      CarrierFailureClass = "timeout"
	CarrierFailureUnavailable  CarrierFailureClass = "unavailable"
	CarrierFailureClosed       CarrierFailureClass = "closed"
)

type carrierFailureError struct {
	class CarrierFailureClass
	cause error
}

func (failure *carrierFailureError) Error() string {
	return "native Node Carrier " + string(failure.class)
}

// CarrierFailureClassOf returns the stable Carrier outcome carried by err.
func CarrierFailureClassOf(err error) (CarrierFailureClass, bool) {
	var failure *carrierFailureError
	if !errors.As(err, &failure) {
		return "", false
	}
	return failure.class, true
}

func carrierFailure(class CarrierFailureClass, cause error) error {
	if cause == nil {
		return nil
	}
	if _, classified := CarrierFailureClassOf(cause); classified {
		return cause
	}
	return &carrierFailureError{class: class, cause: cause}
}

func classifyCarrierFailure(cause error) error {
	if cause == nil {
		return nil
	}
	if _, classified := CarrierFailureClassOf(cause); classified {
		return cause
	}
	class := CarrierFailureUnavailable
	switch {
	case errors.Is(cause, context.Canceled):
		class = CarrierFailureCanceled
	case errors.Is(cause, context.DeadlineExceeded):
		class = CarrierFailureTimeout
	case errors.Is(cause, io.EOF), errors.Is(cause, io.ErrClosedPipe), errors.Is(cause, net.ErrClosed):
		class = CarrierFailureClosed
	default:
		var networkError net.Error
		if errors.As(cause, &networkError) && networkError.Timeout() {
			class = CarrierFailureTimeout
		}
	}
	return carrierFailure(class, cause)
}

type authenticatedNodeCarrier struct {
	mu     sync.Mutex
	lane   Carrier
	closed bool
}

func authenticatedCarrier(lane Carrier) Carrier {
	return &authenticatedNodeCarrier{lane: lane}
}

func (carrier *authenticatedNodeCarrier) Read(buffer []byte) (int, error) {
	lane, err := carrier.openLane()
	if err != nil {
		return 0, err
	}
	count, readErr := lane.Read(buffer)
	return count, classifyCarrierFailure(readErr)
}

func (carrier *authenticatedNodeCarrier) Write(buffer []byte) (int, error) {
	lane, err := carrier.openLane()
	if err != nil {
		return 0, err
	}
	count, writeErr := lane.Write(buffer)
	return count, classifyCarrierFailure(writeErr)
}

func (carrier *authenticatedNodeCarrier) SetDeadline(deadline time.Time) error {
	lane, err := carrier.openLane()
	if err != nil {
		return err
	}
	return classifyCarrierFailure(lane.SetDeadline(deadline))
}

func (carrier *authenticatedNodeCarrier) Close() error {
	carrier.mu.Lock()
	if carrier.closed {
		carrier.mu.Unlock()
		return nil
	}
	carrier.closed = true
	lane := carrier.lane
	carrier.mu.Unlock()
	return classifyCarrierFailure(lane.Close())
}

func (carrier *authenticatedNodeCarrier) openLane() (Carrier, error) {
	carrier.mu.Lock()
	defer carrier.mu.Unlock()
	if carrier.closed {
		return nil, carrierFailure(CarrierFailureClosed, errors.New("native Node Carrier is closed"))
	}
	return carrier.lane, nil
}

var _ Carrier = (*authenticatedNodeCarrier)(nil)
