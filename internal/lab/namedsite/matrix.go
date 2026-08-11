package namedsite

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

func runFixedMatrix(ctx context.Context, runner matrixRunner) (matrixResult, error) {
	result := matrixResult{PositiveTotal: 20, Failures: make(map[string]bool)}
	for attempt := 1; attempt <= result.PositiveTotal; attempt++ {
		if err := ctx.Err(); err != nil {
			return result, matrixOperational(err)
		}
		err := runner.positive(ctx, attempt, 1)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return result, matrixOperational(contextErr)
			}
			if errors.Is(err, errMatrixOperational) {
				return result, err
			}
			if isHardGateFailure(err) {
				return failMatrix(result, "positive attempt failed", true), nil
			}
			if errors.Is(err, errScenarioFailure) {
				return failMatrix(result, "positive attempt failed", false), nil
			}
			return result, matrixOperational(err)
		}
		result.PositivePassed++
	}
	for _, name := range fixedFailureCases {
		if err := ctx.Err(); err != nil {
			return result, matrixOperational(err)
		}
		err := runner.failure(ctx, name)
		result.Failures[name] = err == nil
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return result, matrixOperational(contextErr)
			}
			if errors.Is(err, errMatrixOperational) {
				return result, err
			}
			if !errors.Is(err, errFailureAssertion) {
				return result, matrixOperational(err)
			}
			hard := name == "forbidden_origin_query_role_view" || name == "application_dns_escape" || name == "application_socket_escape" || name == "application_listener_escape"
			return failMatrix(result, "failure condition did not fail closed: "+name, hard), nil
		}
	}
	for episode := 1; episode <= 5; episode++ {
		if err := ctx.Err(); err != nil {
			return result, matrixOperational(err)
		}
		migration, err := runner.migrate(ctx, episode)
		result.Migrations = append(result.Migrations, migration)
		if err != nil || !migration.GenerationOneStopped || !migration.GenerationTwoPassed || !migration.OldInstanceRejected {
			if contextErr := ctx.Err(); contextErr != nil {
				return result, matrixOperational(contextErr)
			}
			if errors.Is(err, errMatrixOperational) {
				return result, err
			}
			if err != nil && !errors.Is(err, errScenarioFailure) {
				return result, matrixOperational(err)
			}
			return failMatrix(result, "migration episode failed", false), nil
		}
	}
	if err := ctx.Err(); err != nil {
		return result, matrixOperational(err)
	}
	result.Verdict = "advance"
	return result, nil
}

var errHardGateFailure = errors.New("gate C hard failure")
var errMatrixOperational = errors.New("gate C matrix operational failure")
var errScenarioFailure = errors.New("gate C scenario failure")

func hardGate(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errHardGateFailure, err)
}

func isHardGateFailure(err error) bool {
	return errors.Is(err, errHardGateFailure)
}

func matrixOperational(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errMatrixOperational, err)
}

func scenarioFailure(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errScenarioFailure, err)
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
		return errors.New("gate C matrix runner is incomplete")
	}
	return nil
}
