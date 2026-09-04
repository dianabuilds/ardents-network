//go:build ignore

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriterLeaseAndEventPublicationAreExclusive(t *testing.T) {
	t.Parallel()
	personaRoot := filepath.Join(t.TempDir(), "honest_user")
	if err := os.MkdirAll(filepath.Join(personaRoot, "events"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(personaRoot, "_meta"), 0o700); err != nil {
		t.Fatal(err)
	}
	first, err := acquireWriter(personaRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer first.release()
	if _, err := acquireWriter(personaRoot); !errors.Is(err, errWriterActive) {
		t.Fatalf("second writer returned %v, want errWriterActive", err)
	}
	event := eventRecord{Schema: eventSchema, RunID: "run-a", Persona: "honest_user", Sequence: 1, Kind: "accept"}
	path, err := publishEvent(filepath.Join(personaRoot, "events"), event)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(personaRoot, "events", "000001.json") {
		t.Fatalf("event path = %q", path)
	}
	if _, err := publishEvent(filepath.Join(personaRoot, "events"), event); !errors.Is(err, errEventExists) {
		t.Fatalf("duplicate event returned %v, want errEventExists", err)
	}
}

func TestEventTemporaryStaysOutsideEvents(t *testing.T) {
	t.Parallel()
	personaRoot := filepath.Join(t.TempDir(), "honest_user")
	eventsDir := filepath.Join(personaRoot, "events")
	metaDir := filepath.Join(personaRoot, "_meta")
	if err := os.MkdirAll(eventsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(metaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	temporary, err := writeEventTemporary(eventsDir, []byte("temporary"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(temporary)
	if filepath.Dir(temporary) != metaDir {
		t.Fatalf("temporary event directory = %q, want %q", filepath.Dir(temporary), metaDir)
	}
	if _, err := readEvents(eventsDir); err != nil {
		t.Fatalf("reader observed publisher temporary: %v", err)
	}
}

func TestReadEventsRejectsUnexpectedNamesAndSequenceGaps(t *testing.T) {
	t.Parallel()
	t.Run("unexpected name", func(t *testing.T) {
		eventsDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(eventsDir, "notes.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readEvents(eventsDir); err == nil {
			t.Fatal("reader accepted an unexpected event filename")
		}
	})
	t.Run("gap", func(t *testing.T) {
		personaRoot := t.TempDir()
		eventsDir := filepath.Join(personaRoot, "events")
		if err := os.Mkdir(eventsDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(personaRoot, "_meta"), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, sequence := range []uint64{1, 3} {
			event := eventRecord{Schema: eventSchema, RunID: "run-a", Persona: "honest_user", Sequence: sequence, Kind: "accept"}
			if _, err := publishEvent(eventsDir, event); err != nil {
				t.Fatal(err)
			}
		}
		events, err := readEvents(eventsDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := nextSequence(events); err == nil {
			t.Fatal("sequence gap was accepted")
		}
	})
}
