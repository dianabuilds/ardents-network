package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func TestSourceParticipationFirstCreationAndRetainedReopen(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	root := filepath.Join(t.TempDir(), "participation")
	s := &networkState{
		config: config{root: "/test-state", localRoles: root, clock: func() time.Time { return now }, retainedHistory: true},
		// These detached facts exercise local duty storage, not Network authentication.
		current: &epoch.Decision{Snapshot: epoch.Snapshot{NodeID: [32]byte{1}, DeclaredFamily: "source-family", ValidUntil: now.Add(time.Hour)}},
	}
	if err := s.retainSourceServer(); err == nil {
		t.Fatal("retained participation unexpectedly initialized a missing root")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("retained refusal created participation root", err)
	}
	s.config.retainedHistory = false
	if err := s.retainSourceServer(); err != nil {
		t.Fatal("initial Source participation refused", err)
	}
	s.config.retainedHistory = true
	if err := s.retainSourceServer(); err == nil {
		t.Fatal("unjoined Source producer adopted")
	}
}

// This is the local producer lifecycle only: detached Epoch facts grant no
// signed State or serving authority, and no listener or process is created.
func TestSourceParticipationRestartRequiresOriginalProducerRemoval(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	root := filepath.Join(t.TempDir(), "participation")
	makeOwner := func() *networkState {
		return &networkState{config: config{root: "/same-state", localRoles: root,
			clock: func() time.Time { return now }, sourceInfo: source.Details{Serving: true}},
			current: &epoch.Decision{Snapshot: epoch.Snapshot{NodeID: [32]byte{1}, DeclaredFamily: "source-family", ValidUntil: now.Add(time.Hour)}}}
	}
	original := makeOwner()
	if err := original.retainSourceServer(); err != nil {
		t.Fatal(err)
	}
	for _, expired := range []bool{false, true} {
		if expired {
			now = now.Add(2 * time.Hour)
		}
		before, err := os.ReadFile(filepath.Join(root, "current"))
		if err != nil {
			t.Fatal(err)
		}
		successor := makeOwner()
		successor.config.retainedHistory = true
		err = successor.retainSourceServer()
		var required *RecoveryRequiredError
		if !errors.As(err, &required) || !errors.Is(err, duty.ErrLocalRoleProducerRetained) {
			t.Fatal("successor failed to retain unresolved producer cause", expired, err)
		}
		after, err := os.ReadFile(filepath.Join(root, "current"))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("refusal changed participation history", err)
		}
	}
	if err := original.releaseSourceServer(); err != nil {
		t.Fatal(err)
	}
	successor := makeOwner()
	successor.config.retainedHistory = true
	if err := successor.retainSourceServer(); err != nil {
		t.Fatal("original removal did not permit fresh producer", err)
	}
	if err := successor.releaseSourceServer(); err != nil {
		t.Fatal(err)
	}
}
