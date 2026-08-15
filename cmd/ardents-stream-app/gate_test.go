package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkloadStartGatePublishesReadinessBeforeRelease(t *testing.T) {
	root := t.TempDir()
	done := make(chan error, 1)
	ready := filepath.Join(root, "client.start.ready")
	release := filepath.Join(root, "client.start.release")
	go func() { done <- waitGateReleaseWithin(ready, release, 0, time.Second) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(ready); err == nil && string(raw) == "0\n" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-done:
		t.Fatalf("gate returned before release: %v", err)
	default:
	}
	if err := os.WriteFile(release, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestGatedWriterStopsAtExactPrecommittedOffset(t *testing.T) {
	var stored, observed uint32
	write := func(value []byte) (int, error) { stored += uint32(len(value)); return len(value), nil }
	gated := gatedWorkloadSequenceWriter(write, []uint32{2 * 16_381},
		func(offset uint32) error { observed = offset; return nil })
	for range 2 {
		if _, err := gated(make([]byte, 16_381)); err != nil {
			t.Fatal(err)
		}
	}
	if stored != 2*16_381 || observed != stored {
		t.Fatalf("stored %d, gate %d; want exact %d", stored, observed, 2*16_381)
	}
	if _, err := gated([]byte{1}); err != nil {
		t.Fatal(err)
	}
}

func TestGatedWriterStopsAtEachSequentialPrecommittedOffset(t *testing.T) {
	var stored uint32
	var observed []uint32
	write := func(value []byte) (int, error) { stored += uint32(len(value)); return len(value), nil }
	gated := gatedWorkloadSequenceWriter(write, []uint32{2 * 16_381, 4 * 16_381, 6 * 16_381},
		func(offset uint32) error { observed = append(observed, offset); return nil })
	for range 7 {
		if _, err := gated(make([]byte, 16_381)); err != nil {
			t.Fatal(err)
		}
	}
	if len(observed) != 3 || observed[0] != 2*16_381 || observed[1] != 4*16_381 || observed[2] != 6*16_381 {
		t.Fatalf("sequential gates differ: %v", observed)
	}
}

func TestGateReadinessIsPublishedAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.ready")
	if err := publishGateReady(path, 176*16_381); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "2883056\n" {
		t.Fatalf("published readiness %q err=%v", raw, err)
	}
	if _, err := os.Stat(path + ".pending"); !os.IsNotExist(err) {
		t.Fatalf("temporary readiness remained: %v", err)
	}
}
