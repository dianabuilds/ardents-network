// Package diagnostics owns diagnostic command parsing and presentation.
// It does not own health computation or event ownership.
package diagnostics

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (a *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderDiagnosticsUsage(a.ctx.Renderer.Out)
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
		output.Writef(a.ctx.Renderer.Err, "ardentsctl diagnostics: unknown subcommand %q\n", args[0])
		renderDiagnosticsUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func (a *Command) diagnosticsSnapshot(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetDiagnostics(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetDiagnosticsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "diagnostics snapshot")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	diag := resp.Msg.GetDiagnostics()
	health := diag.GetHealth()
	output.KV(a.ctx.Renderer.Out, "health", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		output.KV(a.ctx.Renderer.Out, "primary_reason", reason.GetSummary())
	}
	output.KV(a.ctx.Renderer.Out, "recent_events", fmt.Sprintf("%d", len(diag.GetRecentEvents())))
	output.KV(a.ctx.Renderer.Out, "pending_operations", fmt.Sprintf("%d", len(diag.GetPendingOperations())))
	return 0
}

func (a *Command) diagnosticsHealth(ctx context.Context) int {
	return commandctx.RunQuery(ctx, a.ctx, "diagnostics health", func(callCtx context.Context) (*ardentsv1.HealthSummaryResponse, error) {
		resp, err := a.ctx.Client.Service().GetHealthSummary(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetHealthSummaryRequest{}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}, renderDiagnosticsHealthHuman)
}

func renderDiagnosticsHealthHuman(w io.Writer, msg *ardentsv1.HealthSummaryResponse) {
	output.Header(w, "diagnostics health")
	output.Status(w, msg.GetStatus())
	health := msg.GetHealth()
	output.KV(w, "state", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		output.KV(w, "reason", reason.GetSummary())
	}
	if domains := DegradedDomains(health); len(domains) > 0 {
		output.KV(w, "degraded_domains", strings.Join(domains, ", "))
	}
	output.KV(w, "operator_action_required", output.Bool(health.GetOperatorActionRequired()))
}

func (a *Command) diagnosticsPending(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetPendingOperations(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetPendingOperationsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	renderDiagnosticsPendingHuman(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) diagnosticsExplain(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("diagnostics explain", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var req ardentsv1.ExplainFailureRequest
	fs.StringVar(&req.Scope, "scope", "", "failure scope")
	fs.StringVar(&req.ResourceId, "resource-id", "", "resource id")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("scope", req.Scope); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ExplainFailure(callCtx, client.Request(a.ctx.Token, &req))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	renderDiagnosticsExplainHuman(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func renderDiagnosticsPendingHuman(w io.Writer, msg *ardentsv1.PendingOperationsResponse) {
	output.Header(w, "diagnostics pending")
	output.Status(w, msg.GetStatus())
	if len(msg.GetOperations()) == 0 {
		output.Writeln(w, "no pending operations")
		return
	}
	for _, item := range msg.GetOperations() {
		output.KV(w, "operation", item.GetId())
		output.KV(w, "  kind", item.GetKind())
		output.KV(w, "  domain", item.GetDomain())
		output.KV(w, "  resource", item.GetResource())
		output.KV(w, "  state", item.GetState())
		output.KV(w, "  reason", item.GetReason())
		output.KV(w, "  recoverable", output.Bool(item.GetRecoverable()))
		output.KV(w, "  recovery_action", item.GetRecoveryAction())
		output.KV(w, "  started_at", output.Timestamp(item.GetStartedAt()))
		output.KV(w, "  updated_at", output.Timestamp(item.GetUpdatedAt()))
	}
}

func renderDiagnosticsExplainHuman(w io.Writer, msg *ardentsv1.FailureExplanationResponse) {
	output.Header(w, "diagnostics explain")
	output.Status(w, msg.GetStatus())
	explanation := msg.GetExplanation()
	output.KV(w, "scope", explanation.GetScope())
	output.KV(w, "resource_id", explanation.GetResourceId())
	output.KV(w, "state", explanation.GetState())
	if reason := explanation.GetReason(); reason != nil {
		output.KV(w, "reason", reason.GetSummary())
		output.KV(w, "detail", reason.GetDetail())
		output.KV(w, "impact", reason.GetImpact())
		output.KV(w, "recovery", reason.GetRecovery())
		output.KV(w, "operator_action_required", output.Bool(reason.GetOperatorActionRequired()))
	}
	output.KV(w, "impact", explanation.GetImpact())
	output.KV(w, "recovery", explanation.GetRecovery())
	if next := explanation.GetNextSteps(); len(next) > 0 {
		output.KV(w, "next_steps", strings.Join(next, ", "))
	}
}

func DegradedDomains(health *ardentsv1.HealthSnapshot) []string {
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

func (a *Command) diagnosticsEvents(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("diagnostics events", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var req ardentsv1.ListRecentEventsRequest
	var limit int
	fs.IntVar(&limit, "limit", 10, "max number of recent events")
	fs.StringVar(&req.Cursor, "cursor", "", "pagination cursor")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	req.Limit = int32(limit)
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListRecentEvents(callCtx, client.Request(a.ctx.Token, &req))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "diagnostics events")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	output.KV(a.ctx.Renderer.Out, "next_cursor", resp.Msg.GetNextCursor())
	if len(resp.Msg.GetEvents()) == 0 {
		output.Writeln(a.ctx.Renderer.Out, "no recent events")
		return 0
	}
	for _, item := range resp.Msg.GetEvents() {
		output.Event(a.ctx.Renderer.Out, item)
	}
	return 0
}

func renderDiagnosticsUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] diagnostics <snapshot|health|pending|explain|events>")
}

func requireValue(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}
