package recoverysmoke

import (
	"errors"
	"strings"
	"testing"
)

func TestReplacementObserversStartBeforeProcessInspection(t *testing.T) {
	var events []string
	err := orderedReplacementObservation(func() error {
		events = append(events, "observers")
		return nil
	}, func() error {
		events = append(events, "inspection")
		return nil
	})
	if err != nil || len(events) != 2 || events[0] != "observers" || events[1] != "inspection" {
		t.Fatalf("replacement observation order = %v, err=%v", events, err)
	}
	cause := errors.New("observer failed")
	inspected := false
	err = orderedReplacementObservation(func() error { return cause }, func() error {
		inspected = true
		return nil
	})
	if !errors.Is(err, cause) || inspected {
		t.Fatalf("observer failure = %v, inspected=%t", err, inspected)
	}
}

func TestReplacementObservationPreservesPrimaryAndCleanupFailures(t *testing.T) {
	primary := errors.New("inspect proposal")
	traffic := errors.New("remove observer")
	sampler := errors.New("stop sampler")
	err := replacementObservationError(primary, traffic, sampler)
	for _, cause := range []error{primary, traffic, sampler} {
		if !errors.Is(err, cause) {
			t.Fatalf("combined error %v lacks %v", err, cause)
		}
	}
	for _, context := range []string{"cleanup replacement traffic observers", "stop replacement resource sampler"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("combined error %q lacks %q", err, context)
		}
	}
}

func TestReplacementFinalizationConsumesObservationOwnership(t *testing.T) {
	traffic := trafficObservers{ids: [2]string{"client-observer", "publisher-observer"}}
	sampler := &statsSampler{}
	calls := 0
	_, err := finalizeReplacementObservation(&traffic, &sampler,
		func(ownedTraffic *trafficObservers, ownedSampler *statsSampler) (replacementCell, error) {
			calls++
			if ownedSampler == nil {
				t.Fatal("finalizer did not receive sampler ownership")
			}
			ownedTraffic.ids = [2]string{}
			return replacementCell{}, nil
		})
	if err != nil || calls != 1 || traffic.ids != [2]string{} || sampler != nil {
		t.Fatalf("finalization calls=%d traffic=%v sampler=%v err=%v", calls, traffic.ids, sampler, err)
	}
}
