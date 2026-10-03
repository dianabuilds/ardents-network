package forwarding

import (
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// receivingResources is the concrete owner of the receiver
// state that must be initialized as one group before a forwarding server can
// accept work. Pool, listener and host policy remain with startup composition.
type receivingResources struct {
	spends       *spending.Ledger
	limits       *route.ClosedDutyLimits
	bootstrap    *route.ClosedBootstrapController
	closeSpends  func(*spending.Ledger) error
	closeOnce    sync.Once
	closeOutcome error
}

type receivingOpeners struct {
	openSpends   func(string, spending.Binding) (*spending.Ledger, error)
	closeSpends  func(*spending.Ledger) error
	newLimits    func(func() time.Time) (*route.ClosedDutyLimits, error)
	newBootstrap func(func() time.Time) (*route.ClosedBootstrapController, error)
}

func defaultReceivingOpeners() receivingOpeners {
	return receivingOpeners{
		openSpends:   spending.Open,
		closeSpends:  func(spends *spending.Ledger) error { return spends.Close() },
		newLimits:    route.NewClosedDutyLimits,
		newBootstrap: route.NewClosedBootstrapController,
	}
}

func openReceivingResources(root string, binding spending.Binding, clock func() time.Time) (*receivingResources, error) {
	return openReceivingResourcesWith(root, binding, clock, defaultReceivingOpeners())
}

func openReceivingResourcesWith(root string, binding spending.Binding, clock func() time.Time, openers receivingOpeners) (*receivingResources, error) {
	if openers.openSpends == nil || openers.closeSpends == nil || openers.newLimits == nil || openers.newBootstrap == nil {
		return nil, errors.New("closed forwarding receiving resource initialization is unavailable")
	}
	resources := &receivingResources{closeSpends: openers.closeSpends}
	spends, err := openers.openSpends(root, binding)
	if err != nil {
		return nil, err
	}
	resources.spends = spends
	resources.limits, err = openers.newLimits(clock)
	if err != nil {
		return nil, errors.Join(err, resources.Close())
	}
	resources.bootstrap, err = openers.newBootstrap(clock)
	if err != nil {
		return nil, errors.Join(err, resources.Close())
	}
	return resources, nil
}

// Close releases the exact spend-root lease once and retains its result.
func (resources *receivingResources) Close() error {
	if resources == nil {
		return nil
	}
	resources.closeOnce.Do(func() {
		if resources.spends != nil {
			if resources.closeSpends != nil {
				resources.closeOutcome = resources.closeSpends(resources.spends)
			} else {
				resources.closeOutcome = resources.spends.Close()
			}
		}
	})
	return resources.closeOutcome
}
