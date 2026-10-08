//go:build linux

package worker

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestFailedStopStillObservesOriginalDescendantsAndRetainsFailure(t *testing.T) {
	instance, unit, service := cleanupObservation(t)
	owner := &Cleanup{instance: instance}
	stopFailure := errors.New("original manager stop failed")
	reads, stops := 0, 0
	reports := 0
	owner.firstFailure = func(err error) {
		reports++
		if !errors.Is(err, stopFailure) || reads != 1 || stops != 1 {
			t.Fatal("original Stop failure was not reported before descendant join", err)
		}
	}
	err := owner.joinObserved(t.Context(), func(*os.File) (bool, bool, error) {
		reads++
		if reads > 1 && reports != 1 {
			t.Fatal("original descendant observation preceded failure report")
		}
		return false, reads < 3, nil
	}, func(context.Context, string, string) (properties, properties, error) {
		return unit, service, nil
	}, func(context.Context, string, string) error {
		stops++
		return stopFailure
	})
	if !errors.Is(err, stopFailure) || reads != 3 || stops != 1 || reports != 1 {
		t.Fatalf("failed Stop lost original join/failure: reads=%d stops=%d err=%v", reads, stops, err)
	}
	owner.reportFailure(errors.New("later joined failure"))
	if reports != 1 {
		t.Fatal("later failure replaced the original report")
	}
}
