package route

import (
	"errors"
	"sync"
	"time"
)

// closedCarrierRetirement gives the pool and its borrowers the same physical
// close operation. Session failure may close before pool withdrawal; every
// caller joins that operation and observes its original result.
type closedCarrierRetirement struct {
	Carrier
	once    sync.Once
	failure error
}

func (carrier *closedCarrierRetirement) Close() error {
	carrier.once.Do(func() { carrier.failure = carrier.Carrier.Close() })
	return carrier.failure
}

func (carrier *closedCarrierRetirement) SetWriteDeadline(deadline time.Time) error {
	writer, ok := carrier.Carrier.(interface{ SetWriteDeadline(time.Time) error })
	if !ok {
		return errors.New("closed Carrier write deadline unavailable")
	}
	return writer.SetWriteDeadline(deadline)
}
