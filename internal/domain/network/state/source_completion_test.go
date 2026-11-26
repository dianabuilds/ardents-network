package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// This unit exercises original completion and real durable producer custody.
// The closed in-memory background channel represents joined local work only;
// it supplies no listener, signed Network authority or process qualification.
func TestSourceCompletionRetainsProducerUntilOriginalCommandFinishes(t *testing.T) {
	for _, failure := range []string{"none", "command", "state", "retired", "early"} {
		t.Run(failure, func(t *testing.T) {
			s, _ := newControlCommitFixture(t, true)
			s.serverDone = make(chan struct{})
			close(s.serverDone)
			cause := errors.New("original late completion failure")
			if failure == "retired" {
				s.closed = true
				s.terminalErr = cause
			}
			completion, err := s.RetainSourceCompletion()
			if err != nil {
				t.Fatal(err)
			}
			copy := completion
			if _, err := s.RetainSourceCompletion(); err == nil {
				t.Fatal("second completion owner admitted")
			}
			path := filepath.Join(s.config.localRoles, "current")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "early" && completion.Finish(nil) == nil {
				t.Fatal("unjoined State supplied completion")
			}
			if failure == "state" {
				s.serverErr = cause
			}
			closeErr := s.Close()
			if (failure == "state" || failure == "retired") && !errors.Is(closeErr, cause) || failure != "state" && failure != "retired" && closeErr != nil {
				t.Fatal("State join outcome", closeErr)
			}
			// A released State lease is insufficient: the retained producer still
			// refuses a successor while command input/process cleanup is pending.
			successor := &State{config: s.config, current: s.current}
			if err := successor.retainSourceServer(); err == nil {
				t.Fatal("State.Close alone enabled a successor")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("pending completion changed producer", err)
			}
			var commandErr error
			if failure == "command" {
				commandErr = cause
			}
			result := completion.Finish(commandErr)
			if failure == "none" {
				if result != nil {
					t.Fatal(result)
				}
				if err := successor.retainSourceServer(); err != nil {
					t.Fatal("joined command failed to release producer", err)
				}
				// Repeated old completion must not remove the successor's guard.
				if err := copy.Finish(nil); err != nil {
					t.Fatal(err)
				}
				if err := (&State{config: s.config, current: s.current}).retainSourceServer(); err == nil {
					t.Fatal("copied completion removed successor guard")
				}
				if err := successor.releaseSourceServer(); err != nil {
					t.Fatal(err)
				}
			} else {
				if result == nil || failure != "early" && !errors.Is(result, cause) {
					t.Fatal("completion failure lost", result)
				}
				if copy.Finish(nil) != result {
					t.Fatal("late successful retry replaced failed completion")
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("failed completion cleared producer", err)
				}
			}
		})
	}
}

func TestSourceCompletionRefusesAbsentAndNonservingOwners(t *testing.T) {
	if (SourceCompletion{}).Finish(nil) == nil {
		t.Fatal("absent completion accepted")
	}
	s, _ := newControlCommitFixture(t, false)
	if _, err := s.RetainSourceCompletion(); err == nil {
		t.Fatal("nonserving State granted Source completion")
	}
}
