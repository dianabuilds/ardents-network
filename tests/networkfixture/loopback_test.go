package networkfixture

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

func TestLoopbackTransferReobservesAfterJoinedRead(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	rejected := errors.New("authority retired")
	readComplete := make(chan struct{})
	err := TransferLocal(t.Context(), 64, deadline, func() error {
		select {
		case <-readComplete:
			return rejected
		default:
			return nil
		}
	}, func(reader io.Reader, count int64) error {
		if err := DiscardPayload(reader, count); err != nil {
			return err
		}
		close(readComplete)
		return nil
	})
	if !errors.Is(err, rejected) {
		t.Fatalf("completed loopback transfer returned %v, want retirement after reading", err)
	}
	if err := TransferLoopback(t.Context(), 64, time.Now().Add(5*time.Second), func() error { return nil }); err != nil {
		t.Fatalf("work with unchanged authority: %v", err)
	}
}

func TestWorkAuthorityKeepsLastAcceptedObservationFloor(t *testing.T) {
	start := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	initial := admission.AuthorityFacts{NotBefore: start, NotAfter: start.Add(time.Hour)}
	now := start.Add(time.Minute)
	deadline := start.Add(3 * time.Minute)
	observe := WorkAuthority(initial, start, deadline, func() (admission.AuthorityFacts, time.Time, error) {
		return initial, now, nil
	})
	if err := observe(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(-time.Second)
	if err := observe(); err == nil {
		t.Fatal("backward observation retained work authority")
	}
	now = deadline
	if err := observe(); err == nil {
		t.Fatal("work authority accepted the exclusive deadline")
	}
}
