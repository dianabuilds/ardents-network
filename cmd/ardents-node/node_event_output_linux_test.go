//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

func TestNodeEventEmitterBackpressurePreservesDiagnostics(t *testing.T) {
	peer, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close(); _ = peer.Close() })
	fd := int(output.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	var prefix []byte
	chunk := bytes.Repeat([]byte("p"), 2048)
	for {
		n, err := syscall.Write(fd, chunk)
		if n > 0 {
			prefix = append(prefix, chunk[:n]...)
		}
		if errors.Is(err, syscall.EAGAIN) {
			break
		}
		if err != nil || len(prefix) > 1<<20 {
			t.Fatalf("fill output: bytes=%d err=%v", len(prefix), err)
		}
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "lifecycle.json")
	previous := []byte("previous diagnostic\n")
	if err := os.WriteFile(path, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	emit := nodeEventEmitter(output, directory)
	event := node.Event{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "READY", At: time.Unix(1, 0).UTC()}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := emit(ctx, event); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("saturated command emitter: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("failed output replaced diagnostic: %q, %v", got, err)
	}
	drained := make([]byte, len(prefix))
	if _, err := io.ReadFull(peer, drained); err != nil || !bytes.Equal(drained, prefix) {
		t.Fatalf("drain prefix: %v", err)
	}
	if err := emit(nodeEventContext(t), event); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	wire, err := io.ReadAll(peer)
	if err != nil {
		t.Fatal(err)
	}
	const want = "{\"schema\":\"ardents-node-event-v1\",\"kind\":\"lifecycle\",\"state\":\"READY\",\"at\":\"1970-01-01T00:00:01Z\",\"assignment_digest\":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]}\n"
	// Compare wire to the public grammar and to the actual latest diagnostic.
	got, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(wire, got) || !bytes.Equal(wire, []byte(want)) {
		t.Fatalf("resumed command event: wire=%q diagnostic=%q err=%v", wire, got, err)
	}
}
