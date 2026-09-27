//go:build linux

package qualification

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestOpenQualificationReaderStreamsOverlapsBoundedSetup(t *testing.T) {
	started := make(chan int, 2)
	releaseFirst := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := OpenReaderStreams(t.Context(), 2, 2, time.Time{}, 0,
			func(ctx context.Context, index int) (streamqualification.BoundStream, error) {
				started <- index
				if index == 0 {
					select {
					case <-releaseFirst:
					case <-ctx.Done():
						return streamqualification.BoundStream{}, ctx.Err()
					}
				}
				return streamqualification.BoundStream{ID: uint32(index + 1)}, nil
			})
		done <- err
	}()

	seen := map[int]bool{}
	for len(seen) < 2 {
		select {
		case index := <-started:
			seen[index] = true
		case <-time.After(time.Second):
			t.Fatal("second setup did not overlap the blocked first setup")
		}
	}
	close(releaseFirst)
	if err := <-done; err != nil {
		t.Fatalf("overlapped setup failed: %v", err)
	}
}

func TestQualificationPublisherIncompleteErrorReportsProgressAndRegistrationReason(t *testing.T) {
	err := publisherIncompleteError(193)
	message := err.Error()
	for _, want := range []string{"193/256", "producer ended"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestQualificationReaderOpeningDelayStaggersFourReaders(t *testing.T) {
	want := []time.Duration{0, 250 * time.Millisecond, 500 * time.Millisecond, 750 * time.Millisecond}
	for reader, expected := range want {
		if got := ReaderOpeningDelay(reader); got != expected {
			t.Fatalf("Reader %d opening delay = %s, want %s", reader, got, expected)
		}
	}
}
