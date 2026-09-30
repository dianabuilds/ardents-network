package process

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConnectionCapturePreservesDisabledContextAndDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var disabled *ConnectionTrace
	if disabled.Bind(parent) != parent || ReaderTrace(parent) != nil {
		t.Fatal("disabled Context changed")
	}
	disabled.Observe(Admission, parent)(errors.New("private target"))
	disabled.Finish(nil)
	zero := new(ConnectionTrace)
	if zero.Bind(parent) != parent {
		t.Fatal("zero capture enabled")
	}
	zero.Observe(Admission, parent)(nil)
	zero.Finish(nil)
	capture := newConnectionCapture()
	enabled := context.WithValue(parent, sessionCaptureContextKey{}, capture)
	trace := ReaderTrace(enabled)
	if trace != capture || ReaderTrace(enabled) != nil {
		t.Fatal("more than one operation admitted")
	}
	independent, stop := context.WithTimeout(t.Context(), 2*time.Minute)
	defer stop()
	bound := trace.Bind(independent)
	before, _ := independent.Deadline()
	after, _ := bound.Deadline()
	if before != after || bound.Done() != independent.Done() {
		t.Fatal("authority Context changed")
	}
	end := ObserveConnection(bound, Admission)
	end(errors.New("private target and key"))
	end(nil)
	capture.Finish(context.Canceled)
	snapshot := capture.snapshot()
	if snapshot.State != "joined" || snapshot.Outcome != "canceled" || len(snapshot.Records) != 2 || snapshot.Records[1].State != "failed" || snapshot.Records[0].Budget == nil {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	body, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(body), "private") {
		t.Fatal("private error leaked")
	}
	cancel()
	if bound.Err() != nil {
		t.Fatal("binding imported unrelated cancellation")
	}
}

func TestConnectionCaptureExpiresAndBoundsRecords(t *testing.T) {
	now := time.Now()
	capture := &ConnectionTrace{now: func() time.Time { return now }, opened: now}
	trace := ReaderTrace(context.WithValue(t.Context(), sessionCaptureContextKey{}, capture))
	for i := 0; i < 40; i++ {
		trace.Observe(Admission, t.Context())(nil)
	}
	snapshot := capture.snapshot()
	if len(snapshot.Records) != connectionRecordLimit || snapshot.State != "incomplete" || snapshot.Lost != 16 {
		t.Fatalf("bound: %+v", snapshot)
	}
	capture = &ConnectionTrace{now: func() time.Time { return now }, opened: now}
	trace = ReaderTrace(context.WithValue(t.Context(), sessionCaptureContextKey{}, capture))
	end := trace.Observe(ServiceJoin, t.Context())
	now = now.Add(connectionCaptureWindow)
	end(context.DeadlineExceeded)
	trace.Finish(context.DeadlineExceeded)
	snapshot = capture.snapshot()
	if len(snapshot.Records) != 1 || snapshot.State != "incomplete" || snapshot.Lost != 2 || snapshot.Outcome != "deadline" {
		t.Fatalf("expiry: %+v", snapshot)
	}
	if ReaderTrace(context.WithValue(t.Context(), sessionCaptureContextKey{}, capture)) != nil {
		t.Fatal("expired capture rearmed")
	}
}

func TestConnectionCaptureConcurrentClaimsAndSnapshots(t *testing.T) {
	capture := newConnectionCapture()
	ctx := context.WithValue(t.Context(), sessionCaptureContextKey{}, capture)
	ObserveConnection(ctx, ServiceAuthentication)(nil)
	if len(capture.snapshot().Records) != 0 {
		t.Fatal("session leaked operation observations")
	}
	var wait sync.WaitGroup
	var claims int
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			trace := ReaderTrace(ctx)
			if trace != nil {
				mu.Lock()
				claims++
				mu.Unlock()
				trace.Observe(Admission, ctx)(nil)
			}
			_ = capture.snapshot()
		}()
	}
	wait.Wait()
	if claims != 1 {
		t.Fatalf("claims=%d", claims)
	}
	capture.Finish(nil)
	before := capture.snapshot()
	capture.Observe(WorkerLaunch, ctx)(nil)
	after := capture.snapshot()
	if before.State != "joined" || len(before.Records) != len(after.Records) {
		t.Fatal("terminal capture changed")
	}
}

func TestConnectionCancellationCannotHideCleanupFailure(t *testing.T) {
	for _, err := range []error{errors.Join(context.Canceled, errors.New("cleanup failed")), errors.Join(context.DeadlineExceeded, errors.New("private target"))} {
		if connectionOutcome(err) != "failed" {
			t.Fatal("cleanup failure hidden by cancellation")
		}
	}
	if connectionOutcome(errors.Join(context.Canceled, context.DeadlineExceeded)) != "deadline" {
		t.Fatal("deadline classification lost")
	}
}
