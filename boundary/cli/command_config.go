package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) runConfig(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderConfigUsage(a.stdout)
		return 0
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return a.configUsageError("show does not accept positional arguments")
		}
		return a.configShow(ctx)
	case "reload":
		if len(args) != 1 {
			return a.configUsageError("reload does not accept positional arguments")
		}
		return a.configReload(ctx)
	default:
		_, _ = fmt.Fprintf(a.stderr, "ard config: unknown subcommand %q\n", args[0])
		renderConfigUsage(a.stderr)
		return 2
	}
}

func (a *app) configUsageError(message string) int {
	_, _ = fmt.Fprintf(a.stderr, "ard config: %s\n", message)
	renderConfigUsage(a.stderr)
	return 2
}

func (a *app) configShow(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	response, err := a.client.Service().GetEffectiveConfiguration(
		callCtx, client.Request(a.cfg.Token, &ardentsv1.GetEffectiveConfigurationRequest{}),
	)
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, response.Msg)
		return 0
	}
	printHeader(a.stdout, "effective configuration")
	printStatusLine(a.stdout, response.Msg.GetStatus())
	printEffectiveConfiguration(a, response.Msg.GetConfiguration())
	return 0
}

func (a *app) configReload(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	response, err := a.client.Service().ReloadConfiguration(
		callCtx, client.Request(a.cfg.Token, &ardentsv1.ReloadConfigurationRequest{}),
	)
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, response.Msg)
		return 0
	}
	printHeader(a.stdout, "configuration reload")
	printStatusLine(a.stdout, response.Msg.GetStatus())
	result := response.Msg.GetResult()
	printKV(a.stdout, "outcome", result.GetOutcome())
	printKV(a.stdout, "active_generation", fmt.Sprint(result.GetActiveGeneration()))
	printKV(a.stdout, "candidate_generation", fmt.Sprint(result.GetCandidateGeneration()))
	if paths := result.GetRestartRequired(); len(paths) > 0 {
		printKV(a.stdout, "restart_required", joinCSV(paths))
	}
	if paths := result.GetImmutable(); len(paths) > 0 {
		printKV(a.stdout, "immutable", joinCSV(paths))
	}
	return 0
}

func printEffectiveConfiguration(a *app, snapshot *ardentsv1.EffectiveConfigurationSnapshot) {
	printKV(a.stdout, "api_version", snapshot.GetApiVersion())
	printKV(a.stdout, "active_generation", fmt.Sprint(snapshot.GetActiveGeneration()))
	printKV(a.stdout, "candidate_generation", fmt.Sprint(snapshot.GetCandidateGeneration()))
	printKV(a.stdout, "fingerprint", snapshot.GetFingerprint())
	printKV(a.stdout, "last_reload", snapshot.GetLastReloadOutcome())
	if paths := snapshot.GetPendingRestart(); len(paths) > 0 {
		printKV(a.stdout, "pending_restart", joinCSV(paths))
	}
	if effective := snapshot.GetEffective(); effective != nil {
		renderJSON(a.stdout, effective)
	}
}
