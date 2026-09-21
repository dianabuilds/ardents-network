//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTextPublicationPairCancelledAcknowledgementKeepsCleanupOwnerWithoutCurrentCommit(t *testing.T) {
	registered := &textIntroductionRegistration{}
	pair := textPublicationPairLifecycle{pendingRegistration: registered}
	endpoint := &endpoint{}
	owner := &textContext{}
	endpoint.textPublisherOwner = owner
	endpoint.textPublicationLive = true
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := pair.commitAcknowledgedLocked(ctx, registered, time.Now().UTC()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled local commit = %v", err)
	}
	if registered.published || pair.pendingRegistration != registered || pair.registration != nil || pair.previousRegistration != nil {
		t.Fatal("cancelled acknowledgement exposed a partial current pair")
	}
	if endpoint.textPublisherOwner != owner || !endpoint.textPublicationLive {
		t.Fatal("cancelled acknowledgement lost durable publication cleanup owner")
	}
}

func TestTextPublicationPairDrainRejectsLateAcknowledgement(t *testing.T) {
	registered := &textIntroductionRegistration{}
	pair := textPublicationPairLifecycle{pendingRegistration: registered}
	if !pair.beginDrainLocked() {
		t.Fatal("pair refused its first drain transition")
	}
	if err := pair.commitAcknowledgedLocked(t.Context(), registered, time.Now().UTC()); err == nil {
		t.Fatal("late acknowledgement revived draining publication")
	}
	if registered.published || pair.pendingRegistration != registered || pair.registration != nil {
		t.Fatal("late acknowledgement changed draining pair")
	}
}
