package resource_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestGuardProtectsRecoversAndDrains(t *testing.T) {
	var sample resource.Sample
	guard, err := resource.New(resource.Config{
		Profile: "h3-np1-v1", Interval: time.Second,
		Measure: func() (resource.Sample, error) { return sample, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Observe(0, 0, 0); err != nil {
		t.Fatal(err)
	}
	for index := range 3 {
		sample.CPUUsageUsec += 800_000
		observation, observeErr := guard.Observe(0, 0, 0)
		if observeErr != nil {
			t.Fatal(observeErr)
		}
		if observation.Protect != (index == 2) || observation.Drain {
			t.Fatalf("high sample %d = %+v", index, observation)
		}
	}
	for index := range 120 {
		sample.CPUUsageUsec += 1
		observation, observeErr := guard.Observe(0, 0, 0)
		if observeErr != nil {
			t.Fatal(observeErr)
		}
		if observation.Protect != (index < 119) || observation.Drain {
			t.Fatalf("recovery sample %d = %+v", index, observation)
		}
	}
	sample.MemoryBytes = 460 << 20
	observation, err := guard.Observe(0, 0, 0)
	if err != nil || !observation.Protect || !observation.Drain {
		t.Fatalf("emergency = %+v, %v", observation, err)
	}
}

func TestSourceGuardUsesSourceProfileThresholds(t *testing.T) {
	var sample resource.Sample
	guard, err := resource.New(resource.Config{
		Profile: "h3-s-v1", Interval: time.Second,
		Measure: func() (resource.Sample, error) { return sample, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Observe(2, 0, 0); err != nil {
		t.Fatal(err)
	}
	sample.MemoryBytes = 960 << 20
	for index := range 3 {
		observation, observeErr := guard.Observe(2, 0, 0)
		if observeErr != nil {
			t.Fatal(observeErr)
		}
		if observation.Protect != (index == 2) || observation.Drain {
			t.Fatalf("source high sample %d = %+v", index, observation)
		}
	}
	sample.MemoryBytes = 1152 << 20
	observation, err := guard.Observe(2, 0, 0)
	if err != nil || !observation.Drain {
		t.Fatalf("source emergency = %+v, %v", observation, err)
	}
}
