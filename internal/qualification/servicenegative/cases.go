package servicenegative

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) run(ctx context.Context) (map[string]bool, map[string]string) {
	cases := map[string]struct {
		mechanism string
		run       func(context.Context) bool
	}{
		"session-replay":            {"serviceconn-session-replay", value.sessionReplay},
		"principal-substitution":    {"serviceconn-principal-swap", value.principalSubstitution},
		"restart-reuse":             {"serviceconn-endpoint-restart", value.restartReuse},
		"connection-admin":          {"serviceconn-connection-publish", value.connectionAdministration},
		"credential-only":           {"serviceconn-no-admin-session", value.credentialOnly},
		"wrong-target":              {"publication-target-mutation", value.wrongTarget},
		"wrong-key":                 {"instance-proof-key-mismatch", value.wrongKey},
		"expired":                   {"credential-expired-time", value.expired},
		"wrong-network":             {"credential-network-mutation", value.wrongNetwork},
		"stale-generation":          {"publication-old-generation", value.staleGeneration},
		"same-generation-conflict":  {"publication-generation-conflict", value.sameGenerationConflict},
		"not-yet-valid":             {"credential-future-time", value.notYetValid},
		"wrong-capability":          {"credential-capability-mask", value.wrongCapability},
		"malformed-publication":     {"publication-binary-truncation", value.malformedPublication},
		"administration-connection": {"serviceconn-admin-connect", value.administrationConnection},
		"administration-custody":    {"serviceconn-admin-custody", func(ctx context.Context) bool { return value.forbiddenAdministrationAction(ctx, "custody") }},
		"administration-export":     {"serviceconn-admin-export", func(ctx context.Context) bool { return value.forbiddenAdministrationAction(ctx, "export") }},
		"pid-substitution":          {"process-pid-derived-principal", value.pidSubstitution},
		"container-substitution":    {"broker-container-identity-swap", value.containerSubstitution},
		"malformed-ipc-frame":       {"unix-control-malformed-frame", malformedIPCFrame},
		"oversized-ipc-frame":       {"unix-control-oversized-frame", oversizedIPCFrame},
		"partial-ipc-frame":         {"unix-control-partial-eof", partialIPCFrame},
		"slow-ipc-frame":            {"unix-control-stalled-deadline", slowIPCFrame},
		"stale-generation-new-work": {"retired-runtime-new-connection", value.staleGenerationNewWork},
	}
	results, mechanisms := make(map[string]bool, len(cases)), make(map[string]string, len(cases))
	for name, test := range cases {
		results[name], mechanisms[name] = test.run(ctx), test.mechanism
	}
	return results, mechanisms
}

func denied(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "local authorization or policy denial"
}

func targetFailure(result serviceconn.Result, err error) bool {
	return err != nil && result.Class == "service target authentication failure"
}
