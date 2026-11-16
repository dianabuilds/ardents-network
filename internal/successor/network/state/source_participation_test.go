package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
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
	if err := s.retainSourceServer(); err != nil {
		t.Fatal("existing Source participation refused", err)
	}
}
