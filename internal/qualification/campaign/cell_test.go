package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testAdapter struct {
	prepare func(context.Context) error
	arm     func(context.Context) error
	release func(context.Context) (time.Time, error)
	observe func(context.Context) (CellObservation, error)
	freeze  func(context.Context) (FrozenCell, error)
	cleanup func(context.Context) error
}

func (value testAdapter) Prepare(ctx context.Context) error { return value.prepare(ctx) }
func (value testAdapter) Arm(ctx context.Context) error     { return value.arm(ctx) }
func (value testAdapter) Release(ctx context.Context) (time.Time, error) {
	return value.release(ctx)
}
func (value testAdapter) Observe(ctx context.Context) (CellObservation, error) {
	return value.observe(ctx)
}
func (value testAdapter) Freeze(ctx context.Context) (FrozenCell, error) {
	return value.freeze(ctx)
}
func (value testAdapter) Cleanup(ctx context.Context) (json.RawMessage, error) {
	err := value.cleanup(ctx)
	return json.RawMessage(`{"adapter":"test"}`), err
}

func TestCellDoesNotReleaseWorkloadBeforeSamplerReadiness(t *testing.T) {
	var events []string
	armed := false
	adapter := successfulAdapter()
	adapter.prepare = func(context.Context) error { events = append(events, "prepare"); return nil }
	adapter.arm = func(context.Context) error {
		events = append(events, "arm-ready")
		armed = true
		return nil
	}
	adapter.release = func(context.Context) (time.Time, error) {
		if !armed {
			t.Fatal("workload released before sampler readiness")
		}
		events = append(events, "release")
		return time.Now(), nil
	}
	adapter.observe = func(context.Context) (CellObservation, error) {
		events = append(events, "observe")
		return CellObservation{Candidate: candidatePass, TerminalAt: time.Now().Add(time.Nanosecond)}, nil
	}
	adapter.freeze = func(context.Context) (FrozenCell, error) {
		events = append(events, "freeze")
		return FrozenCell{Candidate: candidatePass, Evidence: json.RawMessage(`{"complete":true}`)}, nil
	}
	adapter.cleanup = func(context.Context) error { events = append(events, "cleanup"); return nil }

	receipt, err := RunCell(context.Background(), testInput(t, "ready", "attempt-0001"), adapter)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"prepare", "arm-ready", "release", "observe", "freeze", "cleanup"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("phase order = %v, want %v", events, want)
	}
	if receipt.Candidate != candidatePass || receipt.Observation != observationComplete {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestInfrastructureErrorCannotBecomeCandidateFailure(t *testing.T) {
	adapter := successfulAdapter()
	adapter.arm = func(context.Context) error { return errors.New("sampler unavailable") }
	released := false
	adapter.release = func(context.Context) (time.Time, error) { released = true; return time.Now(), nil }

	receipt, err := RunCell(context.Background(), testInput(t, "infra", "attempt-0001"), adapter)
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Fatal("workload released after failed arm")
	}
	if receipt.Candidate != candidateNotRun || receipt.Observation != observationInvalid ||
		receipt.Cleanup != cleanupComplete {
		t.Fatalf("infrastructure failure misclassified: %+v", receipt)
	}
}

func TestObservedCandidateFailureSurvivesInvalidFreeze(t *testing.T) {
	adapter := successfulAdapter()
	adapter.observe = func(context.Context) (CellObservation, error) {
		return CellObservation{Candidate: candidateFail, Reason: "deadline missed",
			TerminalAt: time.Now().Add(time.Nanosecond)}, nil
	}
	adapter.freeze = func(context.Context) (FrozenCell, error) {
		return FrozenCell{}, errors.New("final sample unavailable")
	}
	receipt, err := RunCell(context.Background(), testInput(t, "failed", "attempt-0001"), adapter)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Candidate != candidateFail || receipt.Observation != observationInvalid ||
		receipt.Cleanup != cleanupComplete ||
		!strings.Contains(receipt.Reason, "deadline missed") ||
		!strings.Contains(receipt.Reason, "final sample unavailable") {
		t.Fatalf("observed candidate failure was rewritten: %+v", receipt)
	}
}

func TestPrepareAndCleanupDelayDoNotChangeActiveNanos(t *testing.T) {
	now := time.Unix(100, 0)
	advance := func(duration time.Duration) { now = now.Add(duration) }
	adapter := successfulAdapter()
	adapter.prepare = func(context.Context) error { advance(10 * time.Second); return nil }
	adapter.arm = func(context.Context) error { advance(time.Second); return nil }
	adapter.release = func(context.Context) (time.Time, error) {
		advance(2 * time.Second)
		return now, nil
	}
	adapter.observe = func(context.Context) (CellObservation, error) {
		advance(3 * time.Second)
		return CellObservation{Candidate: candidatePass, TerminalAt: now}, nil
	}
	adapter.freeze = func(context.Context) (FrozenCell, error) {
		advance(time.Second)
		return FrozenCell{Candidate: candidatePass, Evidence: json.RawMessage(`{"complete":true}`)}, nil
	}
	adapter.cleanup = func(context.Context) error { advance(30 * time.Second); return nil }

	receipt, err := runCell(context.Background(), testInput(t, "clock", "attempt-0001"), adapter,
		func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ActiveNanos != int64(3*time.Second) {
		t.Fatalf("active duration = %s, want 3s", time.Duration(receipt.ActiveNanos))
	}
}

func TestLaterInvalidAttemptDoesNotEraseSuccessfulReceipt(t *testing.T) {
	input := testInput(t, "durable", "attempt-0001")
	first, err := RunCell(context.Background(), input, successfulAdapter())
	if err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(input.ReceiptRoot, "cells", input.CellID, input.AttemptID, "receipt.json")
	before, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}

	input.AttemptID = "attempt-0002"
	broken := successfulAdapter()
	broken.arm = func(context.Context) error { return errors.New("observer did not become ready") }
	second, err := RunCell(context.Background(), input, broken)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || first.Candidate != candidatePass ||
		second.Candidate != candidateNotRun || second.Observation != observationInvalid {
		t.Fatalf("attempt history was not retained: first=%+v second=%+v", first, second)
	}
}

func successfulAdapter() testAdapter {
	return testAdapter{
		prepare: func(context.Context) error { return nil },
		arm:     func(context.Context) error { return nil },
		release: func(context.Context) (time.Time, error) { return time.Now(), nil },
		observe: func(context.Context) (CellObservation, error) {
			return CellObservation{Candidate: candidatePass, TerminalAt: time.Now().Add(time.Nanosecond)}, nil
		},
		freeze: func(context.Context) (FrozenCell, error) {
			return FrozenCell{Candidate: candidatePass, Evidence: json.RawMessage(`{"complete":true}`)}, nil
		},
		cleanup: func(context.Context) error { return nil },
	}
}

func testInput(t *testing.T, cell, attempt string) CellInput {
	t.Helper()
	return CellInput{CellID: cell, AttemptID: attempt, ManifestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ReceiptRoot: t.TempDir()}
}
