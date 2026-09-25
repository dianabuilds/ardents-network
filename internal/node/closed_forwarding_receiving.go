package node

import (
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// closedForwardingReceivingResources is the concrete owner of the receiver
// state that must be initialized as one group before a forwarding server can
// accept work. Pool, listener and host policy remain with startup composition.
type closedForwardingReceivingResources struct {
	spends       *replay.Ledger
	limits       *route.ClosedDutyLimits
	bootstrap    *route.ClosedBootstrapController
	closeSpends  func(*replay.Ledger) error
	closeOnce    sync.Once
	closeOutcome error
}

type closedForwardingReceivingOpeners struct {
	openSpends   func(string, replay.Binding) (*replay.Ledger, error)
	closeSpends  func(*replay.Ledger) error
	newLimits    func(func() time.Time) (*route.ClosedDutyLimits, error)
	newBootstrap func(func() time.Time) (*route.ClosedBootstrapController, error)
}

func defaultClosedForwardingReceivingOpeners() closedForwardingReceivingOpeners {
	return closedForwardingReceivingOpeners{
		openSpends:   replay.Open,
		closeSpends:  func(spends *replay.Ledger) error { return spends.Close() },
		newLimits:    route.NewClosedDutyLimits,
		newBootstrap: route.NewClosedBootstrapController,
	}
}

func openClosedForwardingReceivingResources(root string, binding replay.Binding, clock func() time.Time) (*closedForwardingReceivingResources, error) {
	return openClosedForwardingReceivingResourcesWith(root, binding, clock, defaultClosedForwardingReceivingOpeners())
}

func openClosedForwardingReceivingResourcesWith(root string, binding replay.Binding, clock func() time.Time, openers closedForwardingReceivingOpeners) (*closedForwardingReceivingResources, error) {
	if openers.openSpends == nil || openers.closeSpends == nil || openers.newLimits == nil || openers.newBootstrap == nil {
		return nil, errors.New("closed forwarding receiving resource initialization is unavailable")
	}
	resources := &closedForwardingReceivingResources{closeSpends: openers.closeSpends}
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
func (resources *closedForwardingReceivingResources) Close() error {
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
