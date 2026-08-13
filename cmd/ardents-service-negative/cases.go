package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) run(ctx context.Context) map[string]bool {
	return map[string]bool{
		"session-replay":           value.sessionReplay(ctx),
		"principal-substitution":   value.principalSubstitution(ctx),
		"restart-reuse":            value.restartReuse(ctx),
		"connection-admin":         value.connectionAdministration(ctx),
		"credential-only":          value.credentialOnly(ctx),
		"wrong-target":             value.wrongTarget(ctx),
		"wrong-key":                value.wrongKey(ctx),
		"expired":                  value.expired(ctx),
		"wrong-network":            value.wrongNetwork(ctx),
		"stale-generation":         value.staleGeneration(ctx),
		"same-generation-conflict": value.sameGenerationConflict(ctx),
	}
}

func denied(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "local authorization or policy denial"
}

func targetFailure(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "service target authentication failure"
}
