//go:build linux

package node

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWriteEventAcceptsSystemdJournalSocket(t *testing.T) {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	output := os.NewFile(uintptr(pair[0]), "journal-output")
	peer := os.NewFile(uintptr(pair[1]), "journal-peer")
	t.Cleanup(func() {
		_ = output.Close()
		_ = peer.Close()
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	want := []byte("lifecycle-ready\n")
	if n, err := writeEvent(ctx, output, want); err != nil || n != len(want) {
		t.Fatalf("write journal event: n=%d err=%v", n, err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(peer, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("journal event = %q, want %q", got, want)
	}
}
