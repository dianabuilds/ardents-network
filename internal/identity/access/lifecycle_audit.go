package access

import (
	"ardents/internal/storage"
)

type lifecycleAudit struct {
	correlationID string
	audience      Audience
	action        Action
	principal     string
	deviceID      string
}

func newLifecycleAudit(audience Audience, action Action) lifecycleAudit {
	return lifecycleAudit{correlationID: nextAuditCorrelationID(), audience: audience, action: action}
}

func (a *lifecycleAudit) identify(principal, deviceID string) {
	a.principal = principal
	a.deviceID = deviceID
}

func (a lifecycleAudit) event(outcome, reason string, grantIDs []string) AuditEvent {
	return AuditEvent{
		Outcome: outcome, Reason: reason,
		Principal: a.principal, DeviceID: a.deviceID, Audience: a.audience,
		Actor: a.principal, Effective: a.principal, Action: a.action,
		GrantIDs: append([]string(nil), grantIDs...), CorrelationID: a.correlationID,
	}
}

func (a lifecycleAudit) recordDenied(service *Service, reason string) {
	if a.principal == "" {
		service.RecordDeniedCall(a.audience, a.action, DenialMalformedRequest)
		return
	}
	if service.audit != nil {
		service.audit.RecordIdentityAccess(a.event("denied", reason, nil))
	}
}

func (a lifecycleAudit) commitSuccessfulMutation(tx storage.WriteTransaction, reason string, grantIDs ...string) error {
	return recordAuditOutbox(tx, a.event("accepted", reason, grantIDs))
}
