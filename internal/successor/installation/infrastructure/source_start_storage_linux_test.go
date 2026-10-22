package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These physical controls own only a disposable journal. They do not open the
// first-start producer, provide Release/initial provenance or start any unit.
func TestSourceFirstStartJournalRetainsPhysicalFailureAndPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own-journal")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &sourceStartStorage{ctx: ctx, journal: &sourceBirth{path: path, file: file, info: info}}
	defer s.close()
	if err := s.record("restart-accepted", path, info, ""); !errors.Is(err, ErrBinding) {
		t.Fatalf("unknown/restart record was accepted: %v", err)
	}
	if raw, err := os.ReadFile(path); err != nil || len(raw) != 0 {
		t.Fatalf("refused phase changed journal: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	closed := s.close()
	if !errors.Is(closed, ErrBinding) || !errors.Is(closed, os.ErrClosed) {
		t.Fatalf("Close lost original refusal or actual physical failure: %v", closed)
	}
	if s.close() != closed {
		t.Fatal("repeated Close replaced retained result")
	}
}

func TestSourceFirstStartJournalCancellationCannotAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own-journal")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &sourceStartStorage{ctx: ctx, journal: &sourceBirth{path: path, file: file, info: info}}
	defer s.close()
	cancel()
	if err := s.record("start-attempt", path, nil, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller appended start intent: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, []byte{}) {
		t.Fatalf("canceled caller changed journal: %v", err)
	}
	if err := s.close(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close lost original cancellation: %v", err)
	}
}

func TestSourceStartPendingRefusesUnknownCompletedLookingFirstAttempt(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "source-startup")
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "source-start-pending")
	if err := os.WriteFile(path, []byte("retained original uncertain start"), 0600); err != nil {
		t.Fatal(err)
	}
	selection := &sourceSelection{binding: sourceBinding{Request: SourceRequest{InstallationRoot: root}}}
	// The root-only reader rejects an unprivileged caller before native history
	// inspection. Under root it must refuse the blocker before inspecting any
	// possibly complete-looking records; neither case supplies installed authority.
	wanted := ErrBinding
	if os.Geteuid() != 0 {
		wanted = ErrInput
	}
	if _, err := readSourceStartHistory(selection, 0, strings.Repeat("a", 64), ""); !errors.Is(err, wanted) {
		t.Fatalf("unknown first start refusal = %v, want %v", err, wanted)
	}
	for _, failure := range []error{os.ErrClosed, context.Canceled, errors.New("original sync outcome uncertain")} {
		storage := &sourceStartStorage{ctx: context.Background(), closed: true, terminal: failure, pending: &sourceBirth{path: path}}
		if err := storage.publishStart(); !errors.Is(err, ErrBinding) {
			t.Fatal("failed first-start writer removed original barrier", err)
		}
		if _, err := os.Lstat(path); err != nil {
			t.Fatal("failed first-start barrier was removed", err)
		}
	}
}
