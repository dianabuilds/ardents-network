package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
)

// These units exercise the original handler count and real participation
// journal, not TLS serving or successful installed command completion.
func TestSourcePredecessorRetirementUpdatesOriginalProducer(t *testing.T) {
	s, _ := newControlCommitFixture(t, true)
	previous := duty.Duty{Identity: [32]byte{99}, Family: [32]byte{77}, Class: "direct-source", State: "live", NotAfter: s.config.clock().Add(time.Hour)}
	current := sourceServerDuty(s.current)
	roles, err := duty.OpenOperation(t.Context(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(roles.Replace(sourceProducer("server", s.config.root), []duty.Duty{previous, current}), roles.Close()); err != nil {
		t.Fatal(err)
	}
	s.servingPredecessors = []duty.Duty{previous}
	s.activeSource = 2
	path := filepath.Join(s.config.localRoles, "current")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s.sourceConnectionActive(-1)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || s.activeSource != 1 || len(s.servingPredecessors) != 1 {
		t.Fatal("first joined handler released a still-used predecessor", err)
	}
	for _, retained := range []duty.Duty{previous, current} {
		if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, retained.Identity, retained.Family); err != nil || !held {
			t.Fatal("active handler lost a retained conflict", held, err)
		}
	}
	for _, lastJoined := range []bool{false, true} {
		if lastJoined {
			s.sourceConnectionActive(-1)
			if s.terminalErr != nil || s.closed || s.activeSource != 0 || len(s.servingPredecessors) != 0 {
				t.Fatal("original producer update became a new-lifetime claim", s.terminalErr)
			}
			if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, previous.Identity, previous.Family); err != nil || held {
				t.Fatal("joined predecessor stayed effective", held, err)
			}
			if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, current.Identity, current.Family); err != nil || !held {
				t.Fatal("predecessor retirement removed the current serving guard", held, err)
			}
		}
		// Retiring an old handler is not completion of the original Source.
		successor := &networkState{config: s.config, current: s.current}
		if err := successor.retainSourceServer(); !errors.Is(err, duty.ErrLocalRoleProducerRetained) {
			t.Fatal("handler join admitted a new Source lifetime", lastJoined, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, current.Identity, current.Family); err != nil || held {
		t.Fatal("original joined State left its serving producer", held, err)
	}
}

func TestSourcePredecessorRetirementRefusesLostHistory(t *testing.T) {
	s, _ := newControlCommitFixture(t, true)
	s.servingPredecessors = []duty.Duty{sourceServerDuty(s.current)}
	s.activeSource = 1
	entries, err := os.ReadDir(s.config.localRoles)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(s.config.localRoles, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	s.sourceConnectionActive(-1)
	if s.terminalErr == nil || !s.closed || len(s.servingPredecessors) != 1 {
		t.Fatal("lost participation history did not retire the original owner", s.terminalErr)
	}
	entries, err = os.ReadDir(s.config.localRoles)
	if err != nil || len(entries) != 0 {
		t.Fatal("predecessor retirement recreated lost history", entries, err)
	}
}
