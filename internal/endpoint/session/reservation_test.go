package session

import (
	"path/filepath"
	"testing"
	"time"
)

// The route composition helpers are pure: root independence is a strict-path
// string check, the deadline bound is a minimum, and releasing an absent
// reservation handle is clean. The durable reservation accounting itself is
// exercised by the Linux hosting behavior tests.
func TestIndependentRouteRootsRequireStrictSeparatePaths(t *testing.T) {
	base := t.TempDir()
	entry := filepath.Join(base, "entry")
	hosting := filepath.Join(base, "hosting")
	if !independentRouteRoots(entry, hosting) {
		t.Fatal("independent strict roots refused")
	}
	if !independentRouteRoots(entry) {
		t.Fatal("single strict root refused")
	}
	if independentRouteRoots(entry, entry) {
		t.Fatal("duplicate root accepted")
	}
	if independentRouteRoots(entry, filepath.Join(entry, "child")) {
		t.Fatal("nested root accepted")
	}
	if independentRouteRoots(filepath.Join(entry, "child"), entry) {
		t.Fatal("nested root accepted in reverse order")
	}
	if independentRouteRoots("relative", hosting) {
		t.Fatal("non-strict root accepted")
	}
}

func TestMinRouteDeadlineRetainsEarliestBound(t *testing.T) {
	end := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	earlier := end.Add(-time.Hour)
	later := end.Add(time.Hour)
	if got := minRouteDeadline(end, later, earlier); !got.Equal(earlier) {
		t.Fatal("earliest bound not retained", got)
	}
	if got := minRouteDeadline(end); !got.Equal(end) {
		t.Fatal("absent bounds changed the deadline", got)
	}
	if got := minRouteDeadline(end, later); !got.Equal(end) {
		t.Fatal("later bound moved the deadline", got)
	}
}

func TestReleaseRouteReservationAbsentHandleIsClean(t *testing.T) {
	if err := releaseRouteReservation(nil); err != nil {
		t.Fatal("absent reservation release refused", err)
	}
}
