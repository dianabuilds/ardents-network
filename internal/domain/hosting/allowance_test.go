package hosting

import (
	"math"
	"testing"
	"time"
)

func TestDirectionalCostAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		direction string
		traffic   Traffic
		cost      uint64
		fails     bool
	}{
		{"tx", Traffic{10, 20}, 10, false}, {"rx", Traffic{10, 20}, 20, false}, {"tx+rx", Traffic{10, 20}, 30, false},
		{"tx+rx", Traffic{math.MaxUint64, 0}, math.MaxUint64, false}, {"tx+rx", Traffic{math.MaxUint64, 1}, 0, true},
		{"tx", Traffic{math.MaxUint64, math.MaxUint64}, math.MaxUint64, false}, {"other", Traffic{1, 1}, 0, true},
	} {
		got, err := (Policy{Directions: tc.direction}).cost(tc.traffic)
		if (err != nil) != tc.fails || got != tc.cost {
			t.Fatalf("%+v: %d %v", tc, got, err)
		}
	}
}

func TestObservationNeverWrapsOrLendsReservedBytes(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	p := Policy{Provider: "fixture", Start: now, End: now.Add(time.Hour), Unit: "B", Quantity: math.MaxUint64, Directions: "tx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1}
	for _, tc := range []struct {
		used, reserved uint64
		remaining      uint64
		drain          bool
	}{
		{math.MaxUint64 - 10, 5, 5, false}, {math.MaxUint64, 0, 0, true}, {math.MaxUint64, 1, 0, true},
	} {
		got := (hostingState{Policy: p, Used: tc.used, Reserved: tc.reserved}).observation(now)
		if got.RemainingBytes != tc.remaining || got.Drain != tc.drain {
			t.Fatalf("%+v -> %+v", tc, got)
		}
	}
	p.Unit = "TiB"
	if _, err := p.limit(); err == nil {
		t.Fatal("denomination overflow accepted")
	}
}
