package processresource

import (
	"context"
	"testing"
	"time"
)

func TestProfileAndOriginalCallerRefuseBeforeNativeEffects(t *testing.T) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		if guard, err := Open(ctx, "h3-np1-v1"); guard != nil || err == nil {
			t.Fatal("historical profile admitted native owner")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if guard, err := Open(ctx, Profile); guard != nil || err == nil {
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
	if err := pressureAt(control, now); err != nil {
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
		func(o *Observation) { o.CPUPressure = 0.01 },
		func(o *Observation) { o.MemoryPressure = 0.01 },
		func(o *Observation) { o.IOPressure = 0.01 },
	} {
		observation := control
		change(&observation)
		if pressureAt(observation, now) == nil {
			t.Fatal("required pressure/evidence refusal absent")
		}
	}
}
