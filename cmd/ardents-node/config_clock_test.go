package main

import (
	"testing"
	"time"
)

func TestSourcePlanClockAdvancesFromDeclaredInitialInstant(t *testing.T) {
	at := time.Date(2033, time.March, 4, 5, 6, 7, 0, time.UTC)
	var elapsed time.Duration
	clock := sourcePlanClock(at, func(time.Time) time.Duration { return elapsed })
	if got := clock(); !got.Equal(at) {
		t.Fatalf("initial source plan clock = %v, want %v", got, at)
	}
	elapsed = 2*time.Hour + time.Second
	if got, want := clock(), at.Add(elapsed); !got.Equal(want) {
		t.Fatalf("advanced source plan clock = %v, want %v", got, want)
	}
}
