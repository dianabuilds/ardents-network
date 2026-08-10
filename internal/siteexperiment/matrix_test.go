package siteexperiment

import (
	"context"
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
	result := runFixedMatrix(t.Context(), runner)
	if result.Verdict != "advance" || positiveCalls != 20 || failureCalls != len(fixedFailureCases) || migrationCalls != 5 {
		t.Fatalf("result=%+v calls=%d/%d/%d", result, positiveCalls, failureCalls, migrationCalls)
	}
}

func TestFixedMatrixStopsOnKnowledgeFailure(t *testing.T) {
	t.Parallel()
	runner := matrixRunner{
		positive: func(context.Context, int, uint64) error { return nil },
		failure: func(_ context.Context, name string) error {
			if name == "forbidden_origin_query_role_view" {
				return context.Canceled
			}
			return nil
		},
		migrate: func(context.Context, int) (migrationResult, error) { return migrationResult{}, nil },
	}
	if result := runFixedMatrix(t.Context(), runner); result.Verdict != "stop" {
		t.Fatalf("verdict=%q, want stop", result.Verdict)
	}
}
