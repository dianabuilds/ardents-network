package node

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func TestOriginalFirstFailureProjectionSurvivesCleanupAndCopies(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()
	first := errors.New("private/path/Target/key must never be projected")
	record := failureRecord{started: time.Now().Add(-time.Second), serving: true}
	record.retain(first, ctx, "network")
	record.retain(context.Canceled, ctx, "caller")
	record.retainCleanup(errors.New("independent physical failure"), ctx, "route")
	copy := record.snapshot(first)
	if copy == nil || copy.Phase != "serving" || copy.Cause != "authority-refusal" || copy.ElapsedMS < 1000 || copy.DeadlineRemainingMS == nil {
		t.Fatal("first event context unavailable", copy)
	}
	copy.Cause = "mutated"
	*copy.DeadlineRemainingMS = 0
	later := record.snapshot(first)
	if later == nil || later.DeadlineRemainingMS == nil || later.Cause != "authority-refusal" || *later.DeadlineRemainingMS <= 0 {
		t.Fatal("copy mutated retained terminal facts", later)
	}
	wrapped := &startupFailure{cause: errors.Join(first, errors.New("cleanup")), fact: later}
	projected := StartupFailure(wrapped)
	if !errors.Is(wrapped, first) || projected == nil || projected.Cause != "authority-refusal" {
		t.Fatal("cleanup erased first logical cause")
	}
	raw, err := json.Marshal(StartupFailure(wrapped))
	if err != nil || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "Target") {
		t.Fatal("private error material projected", string(raw), err)
	}
}

func TestPhysicalCleanupWithoutLogicalCauseRemainsSeparate(t *testing.T) {
	record := failureRecord{started: time.Now(), serving: true}
	record.retainCleanup(errors.New("native close failed"), context.Background(), "route")
	fact := record.snapshot(nil)
	if fact == nil || fact.Kind != "physical" || fact.Phase != "cleanup" || fact.Cause != "cleanup-failure" {
		t.Fatal("physical failure became logical refusal", fact)
	}
	record.retain(context.Canceled, context.Background(), "caller")
	if after := record.snapshot(nil); after == nil || after.Kind != "physical" {
		t.Fatal("late cancellation erased physical first event")
	}
}

func TestOriginalFailureProjectionDistinguishesPhaseAndCaller(t *testing.T) {
	for _, test := range []struct {
		cause       error
		owner, want string
	}{{context.Canceled, "caller", "canceled"}, {context.DeadlineExceeded, "caller", "deadline"}, {errors.New("physical"), "route", "physical-role"}} {
		record := failureRecord{started: time.Now()}
		record.retain(test.cause, context.Background(), test.owner)
		fact := record.snapshot(test.cause)
		if fact == nil || fact.Phase != "startup" || fact.Cause != test.want || fact.DeadlineRemainingMS != nil || fact.Resource != nil {
			t.Fatal("unmeasured facts became zero or cause changed", fact)
		}
	}
}

func TestAuthorityDelayRetainsOriginalProcessMeasurement(t *testing.T) {
	now := time.Now()
	observation := processresource.Observation{Started: now.Add(-2 * time.Second), Completed: now.Add(-2 * time.Second), CPUUsec: 123, CPUPressure: 0.54}
	cause := expiredMeasurement(observation)
	record := failureRecord{started: now.Add(-3 * time.Second), serving: true}
	record.retain(cause, context.Background(), "process")
	fact := record.snapshot(cause)
	if fact == nil || fact.Resource == nil || fact.Resource.Cause != "observation-stale" || fact.Resource.Phase != "authority-observation" || fact.Resource.Observation.Completed != observation.Completed || fact.Resource.ElapsedMS != nil {
		t.Fatal("original stale evidence lost or fabricated", fact)
	}
	fact.Resource.Observation.CPUUsec = 0
	if after := record.snapshot(cause); after == nil || after.Resource == nil || after.Resource.Observation.CPUUsec != 123 {
		t.Fatal("copy changed original stale evidence")
	}
}
