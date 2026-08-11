package siteexperiment

import (
	"context"
	"errors"
	"testing"
)

func TestFixedMatrixRequiresEveryPositiveFailureAndMigration(t *testing.T) {
	t.Parallel()
	positiveCalls, failureCalls, migrationCalls := 0, 0, 0
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error { positiveCalls++; return nil },
		failure:  func(context.Context, string) error { failureCalls++; return nil },
		migrate: func(_ context.Context, episode int) (migrationResult, error) {
			migrationCalls++
			return migrationResult{Episode: episode, GenerationOneStopped: true, GenerationTwoPassed: true, OldInstanceRejected: true}, nil
		},
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "advance" || positiveCalls != 20 || failureCalls != len(fixedFailureCases) || migrationCalls != 5 {
		t.Fatalf("result=%+v calls=%d/%d/%d", result, positiveCalls, failureCalls, migrationCalls)
	}
}

func TestFixedMatrixStopsOnPositiveIsolationFailure(t *testing.T) {
	t.Parallel()
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error {
			return hardGate(errors.New("Application escaped its network boundary"))
		},
		failure: func(context.Context, string) error { return nil },
		migrate: func(context.Context, int) (migrationResult, error) {
			return migrationResult{}, nil
		},
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "stop" {
		t.Fatalf("verdict=%q, want stop", result.Verdict)
	}
}

func TestFixedMatrixStopsOnKnowledgeFailure(t *testing.T) {
	t.Parallel()
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error { return nil },
		failure: func(_ context.Context, name string) error {
			if name == "forbidden_origin_query_role_view" {
				return failureAssertion("forbidden role view observed")
			}
			return nil
		},
		migrate: func(context.Context, int) (migrationResult, error) { return migrationResult{}, nil },
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "stop" {
		t.Fatalf("verdict=%q, want stop", result.Verdict)
	}
}

func TestFixedMatrixDoesNotTurnNegativeProbeSetupErrorIntoStop(t *testing.T) {
	t.Parallel()
	want := errors.New("retained evidence unavailable")
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error { return nil },
		failure: func(_ context.Context, name string) error {
			if name == "forbidden_origin_query_role_view" {
				return matrixOperational(want)
			}
			return nil
		},
		migrate: func(context.Context, int) (migrationResult, error) { return migrationResult{}, nil },
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if !errors.Is(err, want) || result.Verdict != "" {
		t.Fatalf("operational negative-probe failure became a verdict: result=%+v err=%v", result, err)
	}
}

func TestFixedMatrixPropagatesOperationalFailureWithoutVerdict(t *testing.T) {
	t.Parallel()
	want := errors.New("progress write failed")
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error { return matrixOperational(want) },
		failure:  func(context.Context, string) error { return nil },
		migrate: func(context.Context, int) (migrationResult, error) {
			return migrationResult{}, nil
		},
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if !errors.Is(err, want) || result.Verdict != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestFixedMatrixClassifiesTypedScenarioFailureAsRedesign(t *testing.T) {
	t.Parallel()
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error {
			return scenarioFailure(errors.New("authenticated workload mismatch"))
		},
		failure: func(context.Context, string) error { return nil },
		migrate: func(context.Context, int) (migrationResult, error) {
			return migrationResult{}, nil
		},
	}
	result, err := runFixedMatrix(t.Context(), runner)
	if err != nil || result.Verdict != "redesign" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
