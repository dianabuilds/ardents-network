//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func TestOpenQualificationReaderStreamsOverlapsBoundedSetup(t *testing.T) {
	started := make(chan int, 2)
	releaseFirst := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := openQualificationReaderStreams(t.Context(), 2, 2, time.Time{}, 0,
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
