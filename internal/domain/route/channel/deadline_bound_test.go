package channel

import (
	"testing"
	"time"
)

// Every Route owner resolves setup and cleanup bounds through this one
// tie-break. Equal wall deadlines with different monotonic readings must keep
// the first argument's exact reading, so the retained original bound keeps its
// identity no matter which owner resolves it.
func TestMinDeadlineKeepsEarlierOriginalBound(t *testing.T) {
	now := time.Now()
	later := now.Add(10 * time.Second)
	if got := MinDeadline(now, later); got != now {
		t.Fatal("earlier first bound not retained", got)
	}
	if got := MinDeadline(later, now); got != now {
		t.Fatal("earlier second bound not retained", got)
	}
	wire := time.Unix(now.Unix(), 0).UTC()
	if got := MinDeadline(wire, later); got != wire {
		t.Fatal("original wire bound not retained", got)
	}
	if got := MinDeadline(time.Time{}, now); !got.IsZero() {
		t.Fatal("zero bound must remain the earliest bound", got)
	}
	if got := MinDeadline(now, time.Time{}); !got.IsZero() {
		t.Fatal("zero bound must remain the earliest bound", got)
	}
}

func TestMinDeadlineEqualWallKeepsFirstExactReading(t *testing.T) {
	live := time.Now()
	// Round(0) strips the monotonic reading while keeping the exact wall
	// instant: equal wall deadlines, different monotonic readings.
	stripped := live.Round(0)
	if !live.Equal(stripped) || live == stripped {
		t.Fatal("fixture lost the equal-wall different-monotonic pair")
	}
	if got := MinDeadline(live, stripped); got != live {
		t.Fatal("equal wall lost the first argument's monotonic reading", got)
	}
	if got := MinDeadline(stripped, live); got != stripped {
		t.Fatal("equal wall lost the first argument's stripped reading", got)
	}
	wire := time.Unix(live.Unix(), 0).UTC()
	if got := MinDeadline(wire, wire.Round(0)); got != wire {
		t.Fatal("equal wire bounds lost the first argument", got)
	}
	// The retained reading still enforces the same instant either way.
	if !MinDeadline(live, stripped).Equal(MinDeadline(stripped, live)) {
		t.Fatal("tie-break changed the enforced instant")
	}
}

func TestMinDeadlineNeverExtendsEitherBound(t *testing.T) {
	base := time.Now()
	readings := []time.Time{
		base.Add(-time.Second),
		base,
		base.Round(0),
		base.Add(time.Nanosecond),
		base.Add(time.Second),
		time.Unix(base.Unix(), 0).UTC(),
	}
	for _, a := range readings {
		for _, b := range readings {
			got := MinDeadline(a, b)
			if got.After(a) || got.After(b) {
				t.Fatal("shared bound extended an original deadline", a, b, got)
			}
			if got != a && got != b {
				t.Fatal("shared bound invented a reading", a, b, got)
			}
		}
	}
}
