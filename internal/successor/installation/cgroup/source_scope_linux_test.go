package cgroup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceScopeOriginalCancellationAcquiresNoKernelObject(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s, err := retainSourceScope(ctx, 0, true)
	if s != nil || err != context.Canceled {
		_ = s.Close()
		t.Fatal("cancelled physical acquisition acquired custody", err)
	}
}

func TestSourceScopeOrdinaryFilesCannotSupplyOriginalJoinOrReviveClose(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "events")
	if err := os.WriteFile(path, []byte("populated 0\nfrozen 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	events, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := events.Stat()
	if err != nil {
		_ = events.Close()
		t.Fatal(err)
	}
	s := &SourceScope{original: &processScope{ctx: t.Context(), pid: uint32(os.Getpid()), kernel: &pin{events: events, identity: info}}}
	copied := *s
	t.Cleanup(func() { _ = s.Close() })
	if joined, err := s.Joined(); joined || err == nil {
		t.Fatal("ordinary empty-file bytes supplied Source join", joined, err)
	}
	if err := events.Close(); err != nil {
		t.Fatal(err)
	}
	first := s.Close()
	if !errors.Is(first, os.ErrClosed) || s.Close() != first || copied.Close() != first {
		t.Fatal("original physical close failure lost", first)
	}
	if joined, err := s.Joined(); joined || err == nil {
		t.Fatal("closed Source scope regained original join", joined, err)
	}
}
