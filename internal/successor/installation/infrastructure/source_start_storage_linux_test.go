package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
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
