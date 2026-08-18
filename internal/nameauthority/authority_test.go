package nameauthority

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func TestPolicyDelayAndActivation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	state := &PolicyState{
		Active: &RecoveryPolicy{
			Version:     1,
			Authorities: []string{"alice", "bob"},
			Threshold:   2,
			Delay:       2 * time.Second,
		},
	}
	activated := ActivatePolicy(state, now)
	if activated == nil || activated.Active == nil {
		t.Fatal("expected active policy before mutation")
	}

	current := &namelease.Record{Name: "mail.example", Authority: "alice", State: "active"}
	op, updated, err := Plan(current, now+1, Request{
		Kind:  "set-recovery-policy",
		Actor: "alice",
		Name:  "mail.example",
		RecoveryPolicy: &RecoveryPolicy{
			Version:     2,
			Authorities: []string{"carol", "dave"},
			Threshold:   1,
			Delay:       3 * time.Second,
		},
	}, Config{DefaultPolicyDelay: 5 * time.Second}, state)
	if err != nil {
		t.Fatalf("plan set policy: %v", err)
	}
	if op.Kind != "" {
		t.Fatalf("expected no record op, got %q", op.Kind)
	}
	if updated.Pending == nil {
		t.Fatal("expected pending policy")
	}
	if activated = ActivatePolicy(updated, now+3); activated.Pending == nil {
		t.Fatal("policy must remain pending before delay elapses")
	}
	if activated = ActivatePolicy(updated, now+6); activated.Active.Version != 2 || activated.Pending != nil {
		t.Fatalf("policy activation mismatch: %+v", activated)
	}
}

func TestAuthorityContinuityAfterTransfer(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	base, err := namelease.Apply(nil, now, namelease.Op{
		Kind:       "claim",
		Name:       "app.example",
		Generation: 1,
		Authority:  "alice",
		Target:     "t1",
	}, namelease.Policy{DefaultLeaseDuration: time.Minute, DefaultGraceDuration: 10 * time.Second})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	transfer, state, err := Plan(&base, now+1, Request{
		Kind:         "transfer",
		Actor:        "alice",
		Name:         "app.example",
		NewAuthority: "bob",
		NewTarget:    "t1",
	}, Config{DefaultLeaseDuration: time.Minute, DefaultGraceDuration: 10 * time.Second}, nil)
	if err != nil {
		t.Fatalf("plan transfer: %v", err)
	}
	if transfer.NewAuthority != "bob" {
		t.Fatalf("unexpected transfer op: %+v", transfer)
	}
	if state != nil {
		t.Fatalf("policy state should remain nil for transfer")
	}

	afterTransfer, err := namelease.Apply(&base, now+1, transfer, namelease.Policy{DefaultLeaseDuration: time.Minute, DefaultGraceDuration: 10 * time.Second})
	if err != nil {
		t.Fatalf("apply transfer: %v", err)
	}
	if afterTransfer.Authority != "bob" {
		t.Fatalf("expected bob as current authority: %+v", afterTransfer)
	}

	_, _, err = Plan(&afterTransfer, now+2, Request{
		Kind:      "renew",
		Actor:     "alice",
		Name:      "app.example",
		NewTarget: "t2",
	}, Config{DefaultLeaseDuration: time.Minute, DefaultGraceDuration: 10 * time.Second}, nil)
	if err == nil {
		t.Fatal("expected renew denied for former authority")
	}
}
