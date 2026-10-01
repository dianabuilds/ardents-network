//go:build linux

package introduction

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTextPublicationPairCancelledAcknowledgementKeepsCleanupOwnerWithoutCurrentCommit(t *testing.T) {
	registered := &Registration{}
	pair := PairLifecycle{pendingRegistration: registered}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := pair.CommitAcknowledgedLocked(ctx, registered, time.Now().UTC()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled local commit = %v", err)
	}
	if registered.published || pair.pendingRegistration != registered || pair.registration != nil || pair.previousRegistration != nil {
		t.Fatal("cancelled acknowledgement exposed a partial current pair")
	}
}

func TestTextPublicationPairDrainRejectsLateAcknowledgement(t *testing.T) {
	registered := &Registration{}
	pair := PairLifecycle{pendingRegistration: registered}
	if !pair.BeginDrainLocked() {
		t.Fatal("pair refused its first drain transition")
	}
	if err := pair.CommitAcknowledgedLocked(t.Context(), registered, time.Now().UTC()); err == nil {
		t.Fatal("late acknowledgement revived draining publication")
	}
	if registered.published || pair.pendingRegistration != registered || pair.registration != nil {
		t.Fatal("late acknowledgement changed draining pair")
	}
}
