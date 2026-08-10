package siteexperiment

import (
	"context"
	"errors"
)

var fixedFailureCases = []string{
	"invalid_name", "absent_name", "modified_name_record", "stale_name_record",
	"wrong_target_descriptor_binding", "ohttp_replay", "ohttp_nonce_mismatch",
	"wrong_instance_credential", "expired_instance_credential", "superseded_instance_credential",
	"service_offline", "route_unavailable", "ambiguous_failure", "application_dns_escape",
	"application_socket_escape", "application_listener_escape", "forbidden_origin_query_role_view",
}

type matrixResult struct {
	PositivePassed int               `json:"positive_passed"`
	PositiveTotal  int               `json:"positive_total"`
	Failures       map[string]bool   `json:"failures"`
	Migrations     []migrationResult `json:"migrations"`
	Verdict        string            `json:"verdict"`
	Failure        string            `json:"failure,omitempty"`
}

type migrationResult struct {
	Episode              int  `json:"episode"`
	GenerationOneStopped bool `json:"generation_one_stopped"`
	GenerationTwoPassed  bool `json:"generation_two_passed"`
	OldInstanceRejected  bool `json:"old_instance_rejected"`
}

type matrixRunner struct {
	positive func(context.Context, int, uint64) error
	failure  func(context.Context, string) error
	migrate  func(context.Context, int) (migrationResult, error)
}

func runFixedMatrix(ctx context.Context, runner matrixRunner) matrixResult {
	result := matrixResult{PositiveTotal: 20, Failures: make(map[string]bool), Verdict: "advance"}
	for attempt := 1; attempt <= result.PositiveTotal; attempt++ {
		err := runner.positive(ctx, attempt, 1)
		if err != nil {
			return failMatrix(result, "positive attempt failed", false)
		}
		result.PositivePassed++
	}
	for _, name := range fixedFailureCases {
		err := runner.failure(ctx, name)
		result.Failures[name] = err == nil
		if err != nil {
			hard := name == "forbidden_origin_query_role_view" || name == "application_dns_escape" || name == "application_socket_escape" || name == "application_listener_escape"
			return failMatrix(result, "failure condition did not fail closed: "+name, hard)
		}
	}
	for episode := 1; episode <= 5; episode++ {
		migration, err := runner.migrate(ctx, episode)
		result.Migrations = append(result.Migrations, migration)
		if err != nil || !migration.GenerationOneStopped || !migration.GenerationTwoPassed || !migration.OldInstanceRejected {
			return failMatrix(result, "migration episode failed", false)
		}
	}
	return result
}

func failMatrix(result matrixResult, message string, hard bool) matrixResult {
	result.Failure = message
	result.Verdict = "redesign"
	if hard {
		result.Verdict = "stop"
	}
	return result
}

func validateMatrixRunner(runner matrixRunner) error {
	if runner.positive == nil || runner.failure == nil || runner.migrate == nil {
		return errors.New("Gate C matrix runner is incomplete")
	}
	return nil
}
