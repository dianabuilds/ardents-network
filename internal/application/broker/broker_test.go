package broker

import (
	"context"
	"testing"
	"time"
)

func TestBrokerBindsOneUseSessionToPrincipalAndSurface(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	value, err := New(Config{ID: [32]byte{1}, Clock: func() time.Time { return now }, Grants: []Grant{
		{Principal: [32]byte{2}, Surface: Connection}, {Principal: [32]byte{3}, Surface: Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := value.Consume(session, [32]byte{2}, Administration); err == nil {
		t.Fatal("Connection grant administered a service")
	}
	if _, err := value.Consume(session, [32]byte{2}, Connection); err == nil {
		t.Fatal("failed cross-surface consume did not invalidate the session")
	}
	session, err = value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := value.Consume(session, [32]byte{2}, Connection); err == nil {
		t.Fatal("Connection capability bypassed its active-session lease")
	}
	session, err = value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	active, receipt, err := value.Activate(context.Background(), session, [32]byte{2}, Connection)
	if err != nil || receipt.Surface != Connection || receipt.Session == [32]byte{} || value.Active() != 1 {
		t.Fatalf("valid session was not activated: receipt=%+v err=%v", receipt, err)
	}
	active.Release()
	if value.Isolation().State() != GenericUnqualified {
		t.Fatal("generic Broker claimed qualified isolation")
	}
}

func TestBrokerDrainAndRevokeInvalidateExactGrantCapabilities(t *testing.T) {
	t.Parallel()
	value, err := New(Config{ID: [32]byte{1}, Grants: []Grant{
		{Principal: [32]byte{2}, Surface: Connection, PermitDrain: true},
		{Principal: [32]byte{3}, Surface: Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.DrainUntil(Connection, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if value.Active() != 0 {
		t.Fatal("drain retained local capabilities")
	}
	if _, err := value.Admit([32]byte{2}, Connection); err == nil {
		t.Fatal("draining broker admitted new work")
	}
	if _, err := value.Consume(capability, [32]byte{2}, Connection); err == nil {
		t.Fatal("drain retained an outstanding capability")
	}
	if err := value.Drain(Administration); err == nil {
		t.Fatal("unpermitted finite drain was accepted")
	}
	admin, err := value.Admit([32]byte{3}, Administration)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.Revoke([32]byte{3}, Administration); err != nil {
		t.Fatal(err)
	}
	if _, err := value.Consume(admin, [32]byte{3}, Administration); err == nil {
		t.Fatal("revoke retained an outstanding capability")
	}
}

func TestActiveSessionOwnsFiniteBudgetAndCancellation(t *testing.T) {
	now := time.Unix(100, 0)
	value, err := New(Config{ID: [32]byte{1}, Clock: func() time.Time { return now }, Grants: []Grant{
		{Principal: [32]byte{2}, Surface: Connection, PermitDrain: true},
		{Principal: [32]byte{3}, Surface: Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	active, receipt, err := value.Activate(context.Background(), capability, [32]byte{2}, Connection)
	if err != nil || receipt.Surface != Connection || active.Context().Err() != nil || value.Active() != 1 {
		t.Fatalf("active Connection session = receipt=%+v active=%v err=%v count=%d", receipt, active, err, value.Active())
	}
	for index := 0; index < maximumAdministrationSessions; index++ {
		if _, err := value.Admit([32]byte{3}, Administration); err != nil {
			t.Fatalf("admit pending session %d: %v", index, err)
		}
	}
	if _, err := value.Admit([32]byte{3}, Administration); err == nil {
		t.Fatal("active session did not consume the finite session budget")
	}
	if err := value.Revoke([32]byte{2}, Connection); err != nil {
		t.Fatal(err)
	}
	if active.Context().Err() == nil || value.Active() != maximumAdministrationSessions {
		t.Fatalf("exact revoke did not cancel active session: context=%v count=%d", active.Context().Err(), value.Active())
	}
	active.Release()
	active.Release()
	if value.Active() != maximumAdministrationSessions {
		t.Fatal("repeated active-session release changed the budget twice")
	}
}

func TestDrainRequiresPermitAndFiniteDeadline(t *testing.T) {
	now := time.Unix(200, 0)
	value, err := New(Config{ID: [32]byte{1}, Clock: func() time.Time { return now }, Grants: []Grant{
		{Principal: [32]byte{2}, Surface: Connection, PermitDrain: true},
		{Principal: [32]byte{3}, Surface: Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	active, _, err := value.Activate(context.Background(), capability, [32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	if err := value.DrainUntil(Connection, time.Time{}); err == nil {
		t.Fatal("drain accepted an absent terminal deadline")
	}
	if err := value.DrainUntil(Administration, now.Add(time.Second)); err == nil {
		t.Fatal("drain accepted a Grant without prior PermitDrain")
	}
	if active.Context().Err() != nil {
		t.Fatal("refused drain changed an active Connection")
	}
	if err := value.DrainUntil(Connection, now); err != nil {
		t.Fatal(err)
	}
	if active.Context().Err() == nil || value.Active() != 0 {
		t.Fatalf("terminal drain boundary did not cancel active work: context=%v count=%d", active.Context().Err(), value.Active())
	}
	if _, err := value.Admit([32]byte{2}, Connection); err == nil {
		t.Fatal("draining Grant admitted new work")
	}
}
