package broker

import (
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
	receipt, err := value.Consume(session, [32]byte{2}, Connection)
	if err != nil || receipt.Surface != Connection || receipt.Session == [32]byte{} || value.Active() != 0 {
		t.Fatalf("valid session was not consumed: receipt=%+v err=%v", receipt, err)
	}
	if value.Isolation().State != "generic/unqualified" {
		t.Fatal("generic Broker claimed qualified isolation")
	}
}

func TestBrokerDrainRejectsNewAndOutstandingCapabilities(t *testing.T) {
	t.Parallel()
	value, err := New(Config{ID: [32]byte{1}, Grants: []Grant{{Principal: [32]byte{2}, Surface: Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := value.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	value.Drain()
	if value.Active() != 0 {
		t.Fatal("drain retained local capabilities")
	}
	if _, err := value.Admit([32]byte{2}, Connection); err == nil {
		t.Fatal("draining broker admitted new work")
	}
	if _, err := value.Consume(capability, [32]byte{2}, Connection); err == nil {
		t.Fatal("drain retained an outstanding capability")
	}
}
