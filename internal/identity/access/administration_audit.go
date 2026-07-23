package access

import (
	"context"
	"time"

	"ardents/internal/storage"
)

// administrationAudit carries the facts learned while a protected
// administration call is re-admitted in its mutation transaction.
type administrationAudit struct {
	correlationID string
	attempted     bool
	session       Session
	trace         admissionTrace
	call          AuthorizedCall
}

func newAdministrationAudit(attempt Attempt) administrationAudit {
	audit := administrationAudit{correlationID: nextAuditCorrelationID()}
	if action, err := ParseAction(attempt.Binding.Audience.Interface, string(attempt.Action)); err == nil {
		audit.trace.Action = action
	}
	return audit
}

func (a *administrationAudit) admit(
	service *Service,
	tx storage.ReadTransaction,
	now time.Time,
	attempt Attempt,
) (AuthorizedCall, error) {
	a.attempted = true
	call, session, err := service.admitInTransactionWithTrace(tx, now, attempt, &a.trace)
	a.session = session
	if err != nil {
		return AuthorizedCall{}, err
	}
	call.deviceID = session.DeviceID
	call.correlationID = a.correlationID
	a.call = call
	return call, nil
}

func (a *administrationAudit) recordDenied(service *Service, reason string, attempt Attempt) {
	// Semantic request validation can fail before the mutation transaction is
	// opened. Authenticate the presented Session in a read snapshot in that
	// case so a protected-call denial still carries Actor/Effective facts from
	// a successful Admit. Mutation paths already populated the trace and do not
	// perform a second admission here.
	if service != nil && service.audit != nil && !a.attempted {
		a.attempted = true
		service.deviceMu.Lock()
		_ = service.grants.database.View(context.Background(), func(tx storage.ReadTransaction) error {
			_, session, err := service.admitInTransactionWithTrace(
				tx,
				canonicalNow(service.clock.Now()),
				attempt,
				&a.trace,
			)
			a.session = session
			return err
		})
		service.deviceMu.Unlock()
	}
	service.recordAdmission("denied", reason, a.correlationID, a.session, attempt.Binding.Audience, a.trace)
}

func (a administrationAudit) commitSuccessfulMutation(tx storage.WriteTransaction, reason string) error {
	return recordAuditOutbox(tx, successfulMutationEvent(a.call, reason))
}
