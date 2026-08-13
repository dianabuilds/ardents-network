package node

import (
	"errors"
	"testing"
	"time"
)

func TestResourceSeriesVerdictPolicy(t *testing.T) {
	healthy := func() *nodeResourceSeries {
		return &nodeResourceSeries{cpuMilli: []uint64{100, 200, 300}, memory: []uint64{1, 2, 3},
			maxFDs: 8, maxSockets: 1, maxPIDs: 8, maxThreads: 8}
	}
	if err := verifyNodeResourceSeries("node1", healthy()); err != nil {
		t.Fatalf("healthy series rejected: %v", err)
	}
	if err := verifyNodeResourceSeries("node1", nil); !errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("missing series error = %v, want invalid", err)
	}
	cadence := healthy()
	cadence.cadenceFailure = true
	if err := verifyNodeResourceSeries("node1", cadence); !errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("cadence error = %v, want invalid", err)
	}
	overCPU := healthy()
	overCPU.cpuMilli = []uint64{900, 900, 900}
	if err := verifyNodeResourceSeries("node1", overCPU); err == nil || errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("CPU error = %v, want candidate failure", err)
	}
	overFD := healthy()
	overFD.maxFDs = 513
	if err := verifyNodeResourceSeries("node1", overFD); err == nil || errors.Is(err, errInvalidNodeCampaign) {
		t.Fatalf("FD error = %v, want candidate failure", err)
	}
}

func TestResourceObservationEnforcesOneSecondCadence(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	sample := func(at time.Time, cpu uint64) nodeResourceSnapshot {
		return nodeResourceSnapshot{Service: "node1", At: at, CPUUsageUsec: cpu, ContainerID: "container", ProcessStart: "start"}
	}
	for _, test := range []struct {
		name string
		gap  time.Duration
	}{
		{"lower edge", 950 * time.Millisecond},
		{"upper edge", 1050 * time.Millisecond},
		{"too early", 949 * time.Millisecond},
		{"too late", 1051 * time.Millisecond},
		{"non-monotonic", -time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
			observer.observeResource(sample(base, 100), nil)
			observer.observeResource(sample(base.Add(test.gap), 200), nil)
			wantFailure := test.gap < 950*time.Millisecond || test.gap > 1050*time.Millisecond
			if observer.resources["node1"].cadenceFailure != wantFailure {
				t.Fatalf("cadence failure = %v, want %v", observer.resources["node1"].cadenceFailure, wantFailure)
			}
		})
	}
}

func TestResourceObservationEnforcesAbsoluteTickError(t *testing.T) {
	for _, delay := range []time.Duration{-51 * time.Millisecond, -50 * time.Millisecond, 50 * time.Millisecond, 51 * time.Millisecond} {
		observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
		observer.observeResource(nodeResourceSnapshot{Service: "node1", At: time.Now(), ContainerID: "container",
			ProcessStart: "start", TickDelayNanos: int64(delay)}, nil)
		wantFailure := delay < -50*time.Millisecond || delay > 50*time.Millisecond
		if observer.resources["node1"].cadenceFailure != wantFailure {
			t.Fatalf("delay %s failure = %v, want %v", delay, observer.resources["node1"].cadenceFailure, wantFailure)
		}
	}
}

func TestExpectedResourceAbsenceStartsNewSeriesSegment(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
	observer.observeResource(nodeResourceSnapshot{Service: "node1", At: base, CPUUsageUsec: 100,
		ContainerID: "before", ProcessStart: "one"}, nil)
	observer.setExpectedAbsence(true, "node1")
	faults := observer.faultSnapshot()
	observer.observeResources(nil, faults)
	observer.observeResource(nodeResourceSnapshot{Service: "node1", At: base.Add(10 * time.Second), CPUUsageUsec: 10,
		ContainerID: "after", ProcessStart: "two"}, faults)
	observer.setExpectedAbsence(false, "node1")
	series := observer.resources["node1"]
	if series.cadenceFailure || len(series.cpuMilli) != 0 {
		t.Fatalf("restart segment = %+v, want clean boundary", series)
	}
}

func TestExpectedAbsencePublishesOneAtomicResourceState(t *testing.T) {
	observer := nodeObserver{activeFaults: make(map[string]bool), resourceReset: make(chan nodeResourceReset, 2)}
	observer.setExpectedAbsence(true, "source1", "source2", "node1", "node2")
	reset := <-observer.resourceReset
	for _, service := range []string{"source1", "source2", "node1", "node2"} {
		if !reset.faults["absence:"+service] {
			t.Fatalf("atomic fault state omitted %s: %v", service, reset.faults)
		}
	}
	select {
	case extra := <-observer.resourceReset:
		t.Fatalf("expected one resource update, got extra %v", extra.faults)
	default:
	}
}

func TestUnexpectedResourceAbsenceOrRestartInvalidatesSeries(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	for _, test := range []struct {
		name string
		next []nodeResourceSnapshot
	}{
		{name: "absence"},
		{name: "restart", next: []nodeResourceSnapshot{{Service: "node1", At: base.Add(time.Second),
			ContainerID: "after", ProcessStart: "two"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
			observer.observeResource(nodeResourceSnapshot{Service: "node1", At: base,
				ContainerID: "before", ProcessStart: "one"}, nil)
			observer.observeResources(test.next, nil)
			if !observer.resources["node1"].cadenceFailure {
				t.Fatal("unexpected process transition did not invalidate the resource series")
			}
		})
	}
}

func TestFirstResourceSampleMustContainEveryExpectedService(t *testing.T) {
	observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
	observer.observeResources([]nodeResourceSnapshot{{Service: "node1", At: time.Now(),
		ContainerID: "container", ProcessStart: "start"}}, nil)
	if !observer.resources["source1"].cadenceFailure {
		t.Fatal("first-sample absence did not invalidate the source1 series")
	}
}

func TestResourceProfileUsesFaultSnapshotFromScheduledTick(t *testing.T) {
	sample := nodeResourceSnapshot{Service: "node2", At: time.Now(), ContainerID: "container", ProcessStart: "start"}
	observer := nodeObserver{resources: make(map[string]*nodeResourceSeries), activeFaults: make(map[string]bool)}
	observer.observeResource(sample, map[string]bool{"cgroup-drift:node2": true})
	if observer.resources["node2"].profileFailure {
		t.Fatal("captured cgroup-drift fault did not exempt its scheduled sample")
	}
	observer = nodeObserver{resources: make(map[string]*nodeResourceSeries),
		activeFaults: map[string]bool{"cgroup-drift:node2": true}}
	observer.observeResource(sample, nil)
	if !observer.resources["node2"].profileFailure {
		t.Fatal("live cgroup-drift state masked an earlier unexpected profile")
	}
}
