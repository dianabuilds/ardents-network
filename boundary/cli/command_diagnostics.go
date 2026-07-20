package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/proto"
)

func (a *app) runDiagnostics(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderDiagnosticsUsage(a.stdout)
		return 0
	}
	switch args[0] {
	case "snapshot":
		return a.diagnosticsSnapshot(ctx)
	case "health":
		return a.diagnosticsHealth(ctx)
	case "pending":
		return a.diagnosticsPending(ctx)
	case "explain":
		return a.diagnosticsExplain(ctx, args[1:])
	case "events":
		return a.diagnosticsEvents(ctx, args[1:])
	default:
		_, _ = fmt.Fprintf(a.stderr, "ard diagnostics: unknown subcommand %q\n", args[0])
		renderDiagnosticsUsage(a.stderr)
		return 2
	}
}

func (a *app) diagnosticsSnapshot(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetDiagnostics(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetDiagnosticsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "diagnostics snapshot")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	diag := resp.Msg.GetDiagnostics()
	health := diag.GetHealth()
	printKV(a.stdout, "health", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		printKV(a.stdout, "primary_reason", reason.GetSummary())
	}
	printKV(a.stdout, "recent_events", fmt.Sprintf("%d", len(diag.GetRecentEvents())))
	printKV(a.stdout, "pending_operations", fmt.Sprintf("%d", len(diag.GetPendingOperations())))
	return 0
}

func (a *app) diagnosticsHealth(ctx context.Context) int {
	if a.cfg.Watch {
		return a.watchSnapshots(ctx, "diagnostics health", func(callCtx context.Context) (proto.Message, error) {
			resp, err := a.client.Service().GetHealthSummary(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetHealthSummaryRequest{}))
			if err != nil {
				return nil, err
			}
			return resp.Msg, nil
		}, func(w io.Writer, msg proto.Message) {
			renderDiagnosticsHealthHuman(w, msg.(*ardentsv1.HealthSummaryResponse))
		})
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetHealthSummary(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetHealthSummaryRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderDiagnosticsHealthHuman(a.stdout, resp.Msg)
	return 0
}

func renderDiagnosticsHealthHuman(w io.Writer, msg *ardentsv1.HealthSummaryResponse) {
	printHeader(w, "diagnostics health")
	printStatusLine(w, msg.GetStatus())
	health := msg.GetHealth()
	printKV(w, "state", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		printKV(w, "reason", reason.GetSummary())
	}
	if domains := degradedDomains(health); len(domains) > 0 {
		printKV(w, "degraded_domains", joinCSV(domains))
	}
	printKV(w, "operator_action_required", boolString(health.GetOperatorActionRequired()))
}

func (a *app) diagnosticsPending(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetPendingOperations(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetPendingOperationsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderDiagnosticsPendingHuman(a.stdout, resp.Msg)
	return 0
}

func (a *app) diagnosticsExplain(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("diagnostics explain", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var req ardentsv1.ExplainFailureRequest
	fs.StringVar(&req.Scope, "scope", "", "failure scope")
	fs.StringVar(&req.ResourceId, "resource-id", "", "resource id")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if err := requireValue("scope", req.Scope); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ExplainFailure(callCtx, client.Request(a.cfg.Token, &req))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderDiagnosticsExplainHuman(a.stdout, resp.Msg)
	return 0
}

func renderDiagnosticsPendingHuman(w io.Writer, msg *ardentsv1.PendingOperationsResponse) {
	printHeader(w, "diagnostics pending")
	printStatusLine(w, msg.GetStatus())
	if len(msg.GetOperations()) == 0 {
		_, _ = fmt.Fprintln(w, "no pending operations")
		return
	}
	for _, item := range msg.GetOperations() {
		printKV(w, "operation", item.GetId())
		printKV(w, "  kind", item.GetKind())
		printKV(w, "  domain", item.GetDomain())
		printKV(w, "  resource", item.GetResource())
		printKV(w, "  state", item.GetState())
		printKV(w, "  reason", item.GetReason())
		printKV(w, "  recoverable", boolString(item.GetRecoverable()))
		printKV(w, "  recovery_action", item.GetRecoveryAction())
		printKV(w, "  started_at", formatProtoTS(item.GetStartedAt()))
		printKV(w, "  updated_at", formatProtoTS(item.GetUpdatedAt()))
	}
}

func renderDiagnosticsExplainHuman(w io.Writer, msg *ardentsv1.FailureExplanationResponse) {
	printHeader(w, "diagnostics explain")
	printStatusLine(w, msg.GetStatus())
	explanation := msg.GetExplanation()
	printKV(w, "scope", explanation.GetScope())
	printKV(w, "resource_id", explanation.GetResourceId())
	printKV(w, "state", explanation.GetState())
	if reason := explanation.GetReason(); reason != nil {
		printKV(w, "reason", reason.GetSummary())
		printKV(w, "detail", reason.GetDetail())
		printKV(w, "impact", reason.GetImpact())
		printKV(w, "recovery", reason.GetRecovery())
		printKV(w, "operator_action_required", boolString(reason.GetOperatorActionRequired()))
	}
	printKV(w, "impact", explanation.GetImpact())
	printKV(w, "recovery", explanation.GetRecovery())
	if next := explanation.GetNextSteps(); len(next) > 0 {
		printKV(w, "next_steps", joinCSV(next))
	}
}

func degradedDomains(health *ardentsv1.HealthSnapshot) []string {
	if health == nil {
		return nil
	}
	out := make([]string, 0, len(health.GetSubsystems()))
	for _, item := range health.GetSubsystems() {
		if item.GetState() != "ready" {
			out = append(out, item.GetDomain())
		}
	}
	return out
}

func (a *app) diagnosticsEvents(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("diagnostics events", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var req ardentsv1.ListRecentEventsRequest
	var limit int
	fs.IntVar(&limit, "limit", 10, "max number of recent events")
	fs.StringVar(&req.Cursor, "cursor", "", "pagination cursor")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	req.Limit = int32(limit)
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListRecentEvents(callCtx, client.Request(a.cfg.Token, &req))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "diagnostics events")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	printKV(a.stdout, "next_cursor", resp.Msg.GetNextCursor())
	if len(resp.Msg.GetEvents()) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "no recent events")
		return 0
	}
	for _, item := range resp.Msg.GetEvents() {
		printEvent(a.stdout, item)
	}
	return 0
}
