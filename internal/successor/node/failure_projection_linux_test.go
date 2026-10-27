//go:build linux

package node

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func TestForwardingWorkRefusalRetainsOriginalWithoutDenyingInvocation(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Minute))
	defer cancel()
	first := errors.New("private token, Target and native path")
	original := &forwarding{done: make(chan struct{})}
	handle := &Forwarding{original: original}
	original.retainWorkRefusal(ctx, "refill", "coverage", time.Now().Add(-time.Second), first)
	var writers sync.WaitGroup
	defer writers.Wait()
	for range 32 {
		writers.Go(func() { original.retainWorkRefusal(ctx, "admission", "receiving", time.Now(), context.Canceled) })
	}
	for range 32 {
		fact, cause := handle.WorkRefusal()
		if cause != first || fact == nil || fact.Kind != "refusal" || fact.Phase != "refill-coverage" || fact.ElapsedMS < 1000 || fact.DeadlineRemainingMS == nil {
			t.Fatal("original failed work observation changed", fact)
		}
		fact.Phase = "mutated"
		*fact.DeadlineRemainingMS = 0
	}
	writers.Wait()
	fact, cause := handle.WorkRefusal()
	encoded, err := json.Marshal(fact)
	if err != nil || strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "Target") || cause != first || fact.Phase != "refill-coverage" || *fact.DeadlineRemainingMS <= 0 {
		t.Fatal("copy or private cause changed diagnostic custody", string(encoded), err)
	}
	if original.denied || original.first != nil || original.result != nil || original.failure.first != nil {
		t.Fatal("channel refusal changed invocation authority or terminal result")
	}
	select {
	case <-original.done:
		t.Fatal("work observation manufactured physical completion")
	default:
	}
	close(original.done)
}

func TestForwardingWorkRefusalAbsenceAndContextRemainDistinct(t *testing.T) {
	original := &forwarding{}
	handle := &Forwarding{original: original}
	original.retainWorkRefusal(t.Context(), "admission", "receiving", time.Now(), nil)
	if fact, cause := handle.WorkRefusal(); fact != nil || cause != nil {
		t.Fatal("successful work manufactured a refusal")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	original.retainWorkRefusal(ctx, "refill", "receiving", time.Now(), ctx.Err())
	if fact, cause := handle.WorkRefusal(); cause != context.Canceled || fact.Cause != "canceled" || fact.Kind != "refusal" || original.denied {
		t.Fatal("original canceled work changed invocation admission or cause", fact)
	}
}

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
	if later.Cause != "authority-refusal" || *later.DeadlineRemainingMS <= 0 {
		t.Fatal("copy mutated retained terminal facts", later)
	}
	wrapped := &startupFailure{cause: errors.Join(first, errors.New("cleanup")), fact: later}
	if !errors.Is(wrapped, first) || StartupFailure(wrapped).Cause != "authority-refusal" {
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
	if fact.Kind != "physical" || fact.Phase != "cleanup" || fact.Cause != "cleanup-failure" {
		t.Fatal("physical failure became logical refusal", fact)
	}
	record.retain(context.Canceled, context.Background(), "caller")
	if record.snapshot(nil).Kind != "physical" {
		t.Fatal("late cancellation erased physical first event")
	}
}

func TestKnownFirstCauseDoesNotWaitForPhysicalJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &forwarding{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retired"), done: make(chan struct{}), failure: failureRecord{started: time.Now(), serving: true}}
	handle := &Forwarding{original: original}
	cancel()
	handle.Stop()
	observed := make(chan *FailureDiagnostic, 1)
	go func() { observed <- handle.FirstFailure() }()
	select {
	case fact := <-observed:
		if fact == nil || fact.Kind != "logical" || fact.Cause != "canceled" || fact.Phase != "serving" {
			t.Error("already known caller cause unavailable", fact)
		}
		close(original.done)
	case <-time.After(50 * time.Millisecond):
		t.Error("known first cause waited for physical join")
		close(original.done)
		<-observed // Join this diagnostic reader even on the RED path.
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
		if fact.Phase != "startup" || fact.Cause != test.want || fact.DeadlineRemainingMS != nil || fact.Resource != nil {
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
	if fact.Resource == nil || fact.Resource.Cause != "observation-stale" || fact.Resource.Phase != "authority-observation" || fact.Resource.Observation.Completed != observation.Completed || fact.Resource.ElapsedMS != nil {
		t.Fatal("original stale evidence lost or fabricated", fact)
	}
	fact.Resource.Observation.CPUUsec = 0
	if record.snapshot(cause).Resource.Observation.CPUUsec != 123 {
		t.Fatal("copy changed original stale evidence")
	}
}
