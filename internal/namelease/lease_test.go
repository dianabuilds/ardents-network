package namelease

import (
	"testing"
	"time"
)

func TestClaimRenewAdvanceLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	policy := Policy{
		DefaultLeaseDuration: 12 * time.Second,
		DefaultGraceDuration: 4 * time.Second,
	}

	claimed, err := Apply(nil, now, Op{
		Kind:       "claim",
		Name:       "site.example",
		Generation: 1,
		Authority:  "alice",
		Target:     "target:v1",
	}, policy)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.State != "active" || claimed.Generation != 1 {
		t.Fatalf("unexpected claim: %+v", claimed)
	}

	renewed, err := Apply(&claimed, now+10, Op{
		Kind:      "renew",
		Name:      "site.example",
		Authority: "alice",
		Target:    "target:v1",
	}, policy)
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if renewed.State != "active" || renewed.Revision != 2 {
		t.Fatalf("unexpected renew: %+v", renewed)
	}

	graced, err := Apply(&renewed, now+23, Op{
		Kind: "advance",
	}, policy)
	if err != nil {
		t.Fatalf("advance to grace: %v", err)
	}
	if graced.State != "grace" || graced.Revision != 3 {
		t.Fatalf("unexpected grace: %+v", graced)
	}

	released, err := Apply(&graced, now+30, Op{
		Kind: "advance",
	}, policy)
	if err != nil {
		t.Fatalf("advance to release: %v", err)
	}
	if released.State != "released" || released.Revision != 4 {
		t.Fatalf("unexpected release: %+v", released)
	}
}

func TestAuthorityTransitionAndRecoveryPending(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	policy := Policy{
		DefaultLeaseDuration: 10 * time.Second,
		DefaultGraceDuration: 4 * time.Second,
	}

	current, err := Apply(nil, now, Op{
		Kind:       "claim",
		Name:       "mail.example",
		Generation: 1,
		Authority:  "alice",
		Target:     "target:v1",
	}, policy)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	recoveredPending, err := Apply(&current, now+1, Op{
		Kind:          "start-recovery",
		Name:          "mail.example",
		Authority:     "alice",
		RecoveryDelay: 3 * time.Second,
	}, policy)
	if err != nil {
		t.Fatalf("start recovery: %v", err)
	}
	if recoveredPending.State != "recovery-pending" {
		t.Fatalf("unexpected recovery state: %+v", recoveredPending)
	}

	completed, err := Apply(&recoveredPending, now+2, Op{
		Kind:         "install-successor",
		Name:         "mail.example",
		Generation:   1,
		NewAuthority: "bob",
	}, policy)
	if err != nil {
		t.Fatalf("install successor: %v", err)
	}
	if completed.State != "active" || completed.Authority != "bob" {
		t.Fatalf("unexpected successor install: %+v", completed)
	}
}

func TestClaimUsesOperationDurations(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	record, err := Apply(nil, now, Op{
		Kind:          "claim",
		Name:          "timed.example",
		Generation:    1,
		Authority:     "alice",
		Target:        "target:v1",
		LeaseDuration: 20 * time.Second,
		GraceDuration: 5 * time.Second,
	}, Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if record.LeaseExpiresAt != now+20 || record.GraceExpiresAt != now+25 {
		t.Fatalf("operation durations were ignored: %+v", record)
	}
}

func TestConflictAndMonotonicity(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	record, err := Apply(nil, now, Op{
		Kind:       "claim",
		Name:       "conflict.example",
		Generation: 1,
		Authority:  "alice",
		Target:     "t1",
	}, Policy{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	conflicted, err := Apply(&record, now+1, Op{
		Kind:            "conflict",
		Name:            "conflict.example",
		ConflictContext: "fork-score:7",
	}, Policy{})
	if err != nil {
		t.Fatalf("conflict: %v", err)
	}
	if conflicted.State != "conflict" {
		t.Fatalf("expected conflict state: %+v", conflicted)
	}

	resolved, err := Apply(&conflicted, now+2, Op{
		Kind:      "resolve-conflict",
		Name:      "conflict.example",
		Authority: "alice",
		Target:    "t1",
	}, Policy{})
	if err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if resolved.State != "active" || resolved.Revision <= conflicted.Revision {
		t.Fatalf("unexpected conflict resolution: %+v", resolved)
	}
}
