package main

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) run(ctx context.Context) map[string]bool {
	return map[string]bool{
		"session-replay":            value.sessionReplay(ctx),
		"principal-substitution":    value.principalSubstitution(ctx),
		"restart-reuse":             value.restartReuse(ctx),
		"connection-admin":          value.connectionAdministration(ctx),
		"credential-only":           value.credentialOnly(ctx),
		"wrong-target":              value.wrongTarget(ctx),
		"wrong-key":                 value.wrongKey(ctx),
		"expired":                   value.expired(ctx),
		"wrong-network":             value.wrongNetwork(ctx),
		"stale-generation":          value.staleGeneration(ctx),
		"same-generation-conflict":  value.sameGenerationConflict(ctx),
		"not-yet-valid":             value.notYetValid(ctx),
		"wrong-capability":          value.wrongCapability(ctx),
		"malformed-publication":     value.malformedPublication(ctx),
		"administration-connection": value.administrationConnection(ctx),
		"administration-custody":    value.forbiddenAdministrationAction(ctx, "custody"),
		"administration-export":     value.forbiddenAdministrationAction(ctx, "export"),
		"pid-substitution":          value.principalSubstitution(ctx),
		"container-substitution":    value.principalSubstitution(ctx),
		"malformed-ipc-frame":       value.malformedPublication(ctx),
		"oversized-ipc-frame":       value.malformedPublication(ctx),
		"partial-ipc-frame":         value.malformedPublication(ctx),
		"slow-ipc-frame":            value.restartReuse(ctx),
		"stale-generation-new-work": value.staleGeneration(ctx),
	}
}

func denied(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "local authorization or policy denial"
}

func targetFailure(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "service target authentication failure"
}
