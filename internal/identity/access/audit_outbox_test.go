package access

import (
	"context"
	"errors"
	"testing"

	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestAdministrationMutationAuditContainsCompleteAuthorizedFacts(t *testing.T) {
	f := newAdminFixture(t)
	events := []AuditEvent{}
	f.service.audit = AuditSinkFunc(func(event AuditEvent) { events = append(events, event) })

	_, err := f.issue("complete-audit", []string{"node.status"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	event := events[0]
	require.Equal(t, "accepted", event.Outcome)
	require.Equal(t, "access_grant_issued", event.Reason)
	require.Equal(t, f.principal, event.Principal)
	require.Equal(t, f.principal, event.Actor)
	require.Equal(t, f.principal, event.Effective)
	require.Equal(t, Action("identity.grant.issue"), event.Action)
	require.Equal(t, f.binding.Audience, event.Audience)
	require.NotEmpty(t, event.DeviceID)
	require.NotEmpty(t, event.GrantIDs)
	require.Empty(t, event.DelegationID)
	require.True(t, validAuditCorrelationID(event.CorrelationID))

	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		count := 0
		require.NoError(t, tx.ForEach(auditOutboxBucket, func(_, _ []byte) error {
			count++
			return nil
		}))
		require.Zero(t, count)
		return nil
	}))
}

func TestAdministrationAuditOutboxSurvivesRestartAndDrains(t *testing.T) {
	f := newAdminFixture(t)
	f.service.audit = nil
	_, err := f.issue("durable-audit", []string{"node.status"})
	require.NoError(t, err)

	require.Equal(t, 1, auditOutboxCount(t, f.database))
	events := []AuditEvent{}
	_, err = NewService(Config{
		Database: f.database,
		Clock:    f.clock,
		Entropy:  &sequentialEntropy{next: 0xe1},
		Audit:    AuditSinkFunc(func(event AuditEvent) { events = append(events, event) }),
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "access_grant_issued", events[0].Reason)
	require.Equal(t, 0, auditOutboxCount(t, f.database))
}

func TestCorruptAdministrationAuditOutboxFailsServiceStartupClosed(t *testing.T) {
	f := newServiceFixture(t)
	key := []byte("c1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		return tx.Put(auditOutboxBucket, key, []byte(`{"version":1,"outcome":"accepted","unknown":true}`))
	}))

	_, err := NewService(Config{
		Database: f.database, Clock: f.clock, Entropy: &sequentialEntropy{next: 0xe2},
		Audit: AuditSinkFunc(func(AuditEvent) {}),
	})
	require.Error(t, err)
	require.Equal(t, 1, auditOutboxCount(t, f.database))
}

func TestAdministrationMutationAndAuditOutboxRollbackTogether(t *testing.T) {
	f := newAdminFixture(t)
	original := f.service.grants.database
	f.service.grants.database = rollbackAfterSuccessfulCallback{Database: original}

	_, err := f.issue("rollback-audit", []string{"node.status"})
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, 0, auditOutboxCount(t, original))

	grantCount := 0
	require.NoError(t, original.View(f.ctx, func(tx storage.ReadTransaction) error {
		return tx.ForEach(grantsBucket, func(_, _ []byte) error {
			grantCount++
			return nil
		})
	}))
	require.Equal(t, 1, grantCount, "only the initial recovery grant may remain")
}

func TestDurableAuditDeliveryFailureLeavesCommittedEventForRetry(t *testing.T) {
	f := newAdminFixture(t)
	f.service.audit = failingDurableAudit{}

	_, err := f.issue("delivery-retry", []string{"node.status"})
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, 1, auditOutboxCount(t, f.database))

	delivered := []AuditEvent{}
	f.service.audit = recordingDurableAudit{events: &delivered}
	require.NoError(t, f.service.flushAuditOutbox(context.Background()))
	require.Len(t, delivered, 1)
	require.Equal(t, "access_grant_issued", delivered[0].Reason)
	require.Equal(t, 0, auditOutboxCount(t, f.database))
}

type failingDurableAudit struct{}

func (failingDurableAudit) RecordIdentityAccess(AuditEvent) {}
func (failingDurableAudit) RecordIdentityAccessDurable(AuditEvent) error {
	return errors.New("injected diagnostics persistence failure")
}

type recordingDurableAudit struct {
	events *[]AuditEvent
}

func (r recordingDurableAudit) RecordIdentityAccess(event AuditEvent) {
	*r.events = append(*r.events, event)
}
func (r recordingDurableAudit) RecordIdentityAccessDurable(event AuditEvent) error {
	r.RecordIdentityAccess(event)
	return nil
}

type rollbackAfterSuccessfulCallback struct {
	storage.Database
}

func (d rollbackAfterSuccessfulCallback) Update(ctx context.Context, callback func(storage.WriteTransaction) error) error {
	sentinel := errors.New("injected post-callback rollback")
	return d.Database.Update(ctx, func(tx storage.WriteTransaction) error {
		if err := callback(tx); err != nil {
			return err
		}
		return sentinel
	})
}

func auditOutboxCount(t *testing.T, database storage.Database) int {
	t.Helper()
	count := 0
	require.NoError(t, database.View(context.Background(), func(tx storage.ReadTransaction) error {
		return tx.ForEach(auditOutboxBucket, func(_, _ []byte) error {
			count++
			return nil
		})
	}))
	return count
}
