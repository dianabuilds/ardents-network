package processguard

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestObservationFreshnessBoundaries(t *testing.T) {
	now := time.Unix(1800000000, 0)
	oneSecondAgo := now.Add(-time.Second)
	for _, tc := range []struct {
		name               string
		started, completed time.Time
		stale              bool
	}{
		{"at both limits", now.Add(-2 * time.Second), oneSecondAgo, false},
		{"took too long", now.Add(-time.Second - time.Nanosecond), now, true},
		{"completed too long ago", now.Add(-2 * time.Second), oneSecondAgo.Add(-time.Nanosecond), true},
		{"completed before start", now, oneSecondAgo, true},
		{"completed in the future", now, now.Add(time.Nanosecond), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateObservation(Observation{Started: tc.started, Completed: tc.completed}, now)
			if !tc.stale && err == nil {
				return
			}
			var refusal *admissionFailure
			if !tc.stale || !errors.As(err, &refusal) || refusal.reason != "observation-stale" {
				t.Fatalf("validateObservation() = %v, want stale = %t", err, tc.stale)
			}
		})
	}
}

func TestProfileAndOriginalCallerRefuseBeforeNativeEffects(t *testing.T) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		if guard, err := Open(ctx, "h3-np1-v1"); guard != nil || err == nil {
			t.Fatal("historical profile admitted native owner")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if guard, err := Open(ctx, ForwardingProfile); guard != nil || err == nil {
		t.Fatal("canceled original caller admitted native owner")
	}
	var empty Guard
	if _, err := empty.Check(); err == nil {
		t.Fatal("empty original owner admitted work")
	}
}

func TestNativePressureAndUnavailableIntervalDenyAdmission(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	control := Observation{Started: now.Add(-time.Millisecond), Completed: now}
	if err := validateObservation(control, now); err != nil {
		t.Fatal("known current policy control refused", err)
	}
	for _, change := range []func(*Observation){
		func(o *Observation) { o.Started = time.Time{} },
		func(o *Observation) { o.Completed = time.Time{} },
		func(o *Observation) { o.Completed = o.Started.Add(2 * time.Second) },
		func(o *Observation) { o.Started = now.Add(-3 * time.Second); o.Completed = now.Add(-2 * time.Second) },
		func(o *Observation) { o.HighEvents = 1 },
		func(o *Observation) { o.EmergencyEvents = 1 },
		func(o *Observation) { o.PIDEvents = 1 },
		func(o *Observation) { o.MemoryBytes = 768 << 20 },
		func(o *Observation) { o.RSSBytes = 768 << 20 },
		func(o *Observation) { o.GoMemoryBytes = 512 << 20 },
		func(o *Observation) { o.FDs = 1024 },
		func(o *Observation) { o.Threads = 256 },
		func(o *Observation) { o.CPUPressure = 1 },
		func(o *Observation) { o.MemoryPressure = 0.01 },
		func(o *Observation) { o.IOPressure = 1 },
	} {
		observation := control
		change(&observation)
		if validateObservation(observation, now) == nil {
			t.Fatal("required pressure/evidence refusal absent")
		}
	}
}

func TestMeasuredBriefDevelopmentStallsPreserveAdmission(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	// Actual captured original/durable-control readings, not fake native
	// admission. This rule replay supplies no process identity or duty.
	for _, observation := range []Observation{
		{CPUPressure: 0.54}, {IOPressure: 0.18},
		{CPUPressure: 0.99, IOPressure: 0.99},
	} {
		observation.Started, observation.Completed = now.Add(-time.Millisecond), now
		if err := validateObservation(observation, now); err != nil {
			t.Fatal("measured brief stall refused", observation.CPUPressure, observation.IOPressure, err)
		}
	}
}
