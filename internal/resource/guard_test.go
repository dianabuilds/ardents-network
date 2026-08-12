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
	if observation.Sample.MemoryBytes != sample.MemoryBytes {
		t.Fatalf("diagnostic sample = %+v, want measured sample", observation.Sample)
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

func TestNodeGuardCoversEveryDeclaredResourceDimension(t *testing.T) {
	cases := map[string]struct {
		sample     resource.Sample
		timers     uint64
		queueItems uint64
		queueBytes uint64
	}{
		"memory":         {sample: resource.Sample{MemoryBytes: 384 << 20}},
		"managed memory": {sample: resource.Sample{GoMemoryBytes: 288 << 20}},
		"socket memory":  {sample: resource.Sample{SocketMemoryBytes: 128 << 20}},
		"descriptors":    {sample: resource.Sample{FDs: 410}},
		"goroutines":     {sample: resource.Sample{Goroutines: 410}},
		"threads":        {sample: resource.Sample{Threads: 64}},
		"timers":         {timers: 410},
		"queue items":    {queueItems: 410},
		"queue bytes":    {queueBytes: (8 << 20) * 8 / 10},
		"CPU PSI":        {sample: resource.Sample{CPUPressure: 20}},
		"memory PSI":     {sample: resource.Sample{MemoryPressure: 5}},
		"I/O PSI":        {sample: resource.Sample{IOPressure: 1}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			guard, err := resource.New(resource.Config{Profile: "h3-np1-v1", Interval: time.Second,
				Measure: func() (resource.Sample, error) { return test.sample, nil }})
			if err != nil {
				t.Fatal(err)
			}
			var observation resource.Observation
			for range 3 {
				observation, err = guard.Observe(test.timers, test.queueItems, test.queueBytes)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !observation.Protect || observation.Drain {
				t.Fatalf("observation = %+v, want PROTECT", observation)
			}
		})
	}
}
