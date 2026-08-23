package route

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

func TestOpenNativeAttachmentRefusesBeforeEntryAcquisition(t *testing.T) {
	called := false
	source := entryAcquirerFunc(func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
		called = true
		return nil, nil, errors.New("Entry acquisition must not run")
	})
	denied := errors.New("resource admission refused")
	_, err := OpenNativeAttachment(context.Background(), NativeAttachmentRequest{Entry: source,
		NetworkID: [32]byte{1}, Digest: [32]byte{2}, Epoch: 1, Deadline: time.Now().Add(time.Minute),
		Admit: func(context.Context) error { return denied }})
	if !errors.Is(err, denied) {
		t.Fatalf("resource refusal = %v", err)
	}
	if called {
		t.Fatal("Entry acquisition ran after resource refusal")
	}
}

func TestNativeAttachmentCloseRefusesSecondRelease(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	attachment := &NativeAttachment{Connection: left, close: func() error { return nil }}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if err := attachment.Close(); err == nil {
		t.Fatal("second attachment close was accepted")
	}
}
