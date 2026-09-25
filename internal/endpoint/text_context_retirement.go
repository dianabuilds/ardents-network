//go:build linux

package endpoint

import (
	"errors"
)

// textContextRetirement is one concrete snapshot of every child that was
// revoked while textContext.mu was held. It contains no admission path: after
// stopTextContextChildrenLocked returns, the context is closed and every
// captured child has already received cancellation.
type textContextRetirement struct {
	refresh             *textPublicationRefreshRetirement
	publication         *textPublicationPairRetirement
	registrationOpening *textRegistrationFlight
	introduction        *textIntroductionPrefixRetirement
	responder           *textResponderPrefixRetirement
	source              *textSourceRetirement
	issuance            *textIssuanceOperation
	resolution          *textResolutionFlight
	withdrawal          *textSourceFlight
	exchanges           []*textIntroductionExchange
	job                 *textJobRetirement
}

// stopTextContextChildrenLocked revokes every child before any join. The
// caller holds textContext.mu. Maps and flights cleared directly here remain
// textContext-owned; extracted owners clear their own state through stop.
func (owner *textContext) stopTextContextChildrenLocked() *textContextRetirement {
	retirement := &textContextRetirement{}
	retirement.refresh = owner.refresh.stopAsync()
	retirement.publication = owner.textPublicationPairLifecycle.stopLocked()
	owner.signalTextRegistrationsLocked()
	retirement.exchanges = make([]*textIntroductionExchange, 0, len(owner.introductionExchanges))
	for exchange := range owner.introductionExchanges {
		if !exchange.retained {
			exchange.cancel()
		}
		retirement.exchanges = append(retirement.exchanges, exchange)
	}
	retirement.withdrawal = owner.withdrawal
	if retirement.withdrawal != nil {
		retirement.withdrawal.cancel()
	}
	retirement.registrationOpening = owner.registrationOpening
	retirement.registrationOpening.stop()
	owner.introductionDispatch.stopLocked()
	owner.introductionAdmission.stopLocked()
	owner.descriptorHistory.Clear()
	owner.sourceSet = nil
	retirement.introduction = owner.introduction.stopLocked()
	retirement.responder = owner.responder.stopLocked()
	retirement.source = owner.source.stopLocked()
	owner.clearTextPermissionLocked()
	retirement.issuance = owner.issuance
	retirement.issuance.cancel()
	retirement.resolution = owner.resolution
	if retirement.resolution != nil {
		retirement.resolution.cancel()
	}
	retirement.job = owner.job.stopLocked(owner)
	return retirement
}

// join preserves the existing dependency order. Opening operations finish
// before their Route prefixes; registration and refresh producers finish
// before registrations close; issuance/resolution finish before the
// context-owned exchanges; the Job joins last so its root reservation and
// first cleanup error survive until every child is terminal.
//
// No step runs under textContext.mu.
func (retirement *textContextRetirement) join() error {
	if retirement == nil {
		return nil
	}
	retirement.source.joinOpening()
	retirement.introduction.joinOpening()
	retirement.responder.joinOpening()
	retirement.registrationOpening.join()
	// Refresh terminal causes are published by their own owner. Context
	// shutdown must join that owner, but only resource cleanup failures belong
	// in the Context Close result.
	_ = retirement.refresh.join()
	var outcome error
	outcome = errors.Join(outcome, retirement.publication.join())
	outcome = errors.Join(outcome, retirement.introduction.closePrefix())
	outcome = errors.Join(outcome, retirement.responder.closePrefix())
	outcome = errors.Join(outcome, retirement.source.closePrefix())
	retirement.issuance.join()
	if retirement.resolution != nil {
		<-retirement.resolution.done
	}
	if retirement.withdrawal != nil {
		<-retirement.withdrawal.done
	}
	for _, exchange := range retirement.exchanges {
		<-exchange.done
	}
	outcome = errors.Join(outcome, retirement.job.join())
	return outcome
}
