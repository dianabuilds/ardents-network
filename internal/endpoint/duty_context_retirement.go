//go:build linux

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/publication"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokens"
)

// dutyContextRetirement is one concrete snapshot of every child that was
// revoked while dutyContext.mu was held. It contains no admission path: after
// stopDutyContextChildrenLocked returns, the context is closed and every
// captured child has already received cancellation.
type dutyContextRetirement struct {
	refresh             *publication.RefreshRetirement
	publication         *introduction.PairRetirement
	registrationOpening introduction.FlightRef
	introduction        *rolePrefixRetirement
	responder           *rolePrefixRetirement
	source              *source.Retirement
	issuance            *tokens.Operation
	resolution          *resolutionFlight
	withdrawal          introduction.FlightRef
	exchanges           []*introduction.Exchange
	job                 *jobRetirement
}

// stopDutyContextChildrenLocked revokes every child before any join. The
// caller holds dutyContext.mu. Maps and flights cleared directly here remain
// dutyContext-owned; extracted owners clear their own state through stop.
func (owner *dutyContext) stopDutyContextChildrenLocked() *dutyContextRetirement {
	retirement := &dutyContextRetirement{}
	retirement.refresh = owner.publication.refresh.StopAsync()
	retirement.publication = owner.publication.pair.StopLocked()
	owner.publication.signalRegistrationsLocked()
	retirement.exchanges = owner.introduction.exchanges.StopLocked()
	retirement.withdrawal = owner.publication.pair.WithdrawalLocked()
	if retirement.withdrawal != nil {
		retirement.withdrawal.CancelFlight()
	}
	retirement.registrationOpening = owner.publication.pair.OpeningLocked()
	if retirement.registrationOpening != nil {
		retirement.registrationOpening.CancelFlight()
	}
	owner.introduction.dispatch.StopLocked()
	owner.introduction.admission.StopLocked()
	owner.descriptorHistory.Clear()
	retirement.introduction = owner.introduction.prefix.stopLocked()
	retirement.responder = owner.responder.stopLocked()
	retirement.source = owner.source.StopLocked()
	owner.sourceSet = nil
	owner.tokens.ClearPermissionLocked()
	retirement.issuance = owner.tokens.Issuance
	retirement.issuance.Cancel()
	retirement.resolution = owner.resolution.StopLocked()
	retirement.job = owner.job.stopLocked(owner)
	return retirement
}

// join preserves the existing dependency order. Opening operations finish
// before their Route prefixes; registration and refresh producers finish
// before registrations close; issuance/resolution finish before the
// context-owned exchanges; the Job joins last so its root reservation and
// first cleanup error survive until every child is terminal.
//
// No step runs under dutyContext.mu.
func (retirement *dutyContextRetirement) join() error {
	if retirement == nil {
		return nil
	}
	retirement.source.JoinOpening()
	retirement.introduction.joinOpening()
	retirement.responder.joinOpening()
	if retirement.registrationOpening != nil {
		retirement.registrationOpening.JoinFlight()
	}
	// Refresh terminal causes are published by their own owner. Context
	// shutdown must join that owner, but only resource cleanup failures belong
	// in the Context Close result.
	_ = retirement.refresh.Join()
	var outcome error
	outcome = errors.Join(outcome, retirement.publication.Join())
	outcome = errors.Join(outcome, retirement.introduction.closePrefix())
	outcome = errors.Join(outcome, retirement.responder.closePrefix())
	outcome = errors.Join(outcome, retirement.source.ClosePrefix())
	retirement.issuance.Join()
	retirement.resolution.Join()
	if retirement.withdrawal != nil {
		retirement.withdrawal.JoinFlight()
	}
	for _, exchange := range retirement.exchanges {
		<-exchange.Done()
	}
	outcome = errors.Join(outcome, retirement.job.join())
	return outcome
}
