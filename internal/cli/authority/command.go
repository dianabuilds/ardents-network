// Package authority owns Realm Authority CLI parsing and redacted presentation.
// It does not own authority state, policy, or signer custody.
package authority

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"ardents/internal/authority"
	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (c *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderUsage(c.ctx.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "create":
		return c.create(ctx, args[1:])
	case "inspect":
		return c.inspect(ctx, args[1:])
	case "delivery":
		return c.delivery(ctx, args[1:])
	case "rotation":
		return c.rotation(ctx, args[1:])
	case "membership":
		return c.membership(ctx, args[1:])
	default:
		output.Writef(c.ctx.Renderer.Err, "ardentsctl authority: unknown subcommand %q\n", args[0])
		renderUsage(c.ctx.Renderer.Err)
		return 2
	}
}

func renderUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] authority <create|inspect|delivery|rotation|membership>")
}

func (c *Command) create(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority create", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var requestID string
	fs.StringVar(&requestID, "request-id", "", "stable idempotency identity")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if requestID == "" || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().CreateRealmAuthority(callCtx, client.Request(&ardentsv1.CreateRealmAuthorityRequest{
		Version: authority.ContractVersion, RequestId: requestID, RealmClass: authority.RealmClassProduction,
	}))
	if err != nil {
		return c.ctx.Failure(err)
	}
	if c.ctx.Renderer.JSON {
		output.JSON(c.ctx.Renderer.Out, response.Msg)
		return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
	}
	output.Header(c.ctx.Renderer.Out, "authority create")
	output.Status(c.ctx.Renderer.Out, response.Msg.GetStatus())
	renderStatus(c.ctx.Renderer.Out, response.Msg.GetAuthority())
	output.KV(c.ctx.Renderer.Out, "operation_id", response.Msg.GetOperationId())
	return c.ctx.Renderer.MutationOutcome(response.Msg.GetStatus())
}

func (c *Command) inspect(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("authority inspect", flag.ContinueOnError)
	fs.SetOutput(c.ctx.Renderer.Err)
	var realmID string
	fs.StringVar(&realmID, "realm-id", "", "exact Realm identifier")
	if err := fs.Parse(args); err != nil {
		return c.ctx.Failure(err)
	}
	if realmID == "" || fs.NArg() != 0 {
		return c.ctx.Failure(flag.ErrHelp)
	}
	callCtx, cancel := c.ctx.Call(ctx)
	defer cancel()
	response, err := c.ctx.Client.Service().InspectRealmAuthority(callCtx, client.Request(&ardentsv1.InspectRealmAuthorityRequest{
		Version: authority.ContractVersion, RealmId: realmID,
	}))
	if err != nil {
		return c.ctx.Failure(err)
	}
	if c.ctx.Renderer.JSON {
		output.JSON(c.ctx.Renderer.Out, response.Msg)
		return 0
	}
	output.Header(c.ctx.Renderer.Out, "authority inspect")
	output.Status(c.ctx.Renderer.Out, response.Msg.GetStatus())
	renderStatus(c.ctx.Renderer.Out, response.Msg.GetAuthority())
	return 0
}

func renderStatus(writer io.Writer, status *ardentsv1.AuthorityStatusSnapshot) {
	if status == nil {
		return
	}
	output.KV(writer, "realm_id", status.GetRealmId())
	output.KV(writer, "realm_class", status.GetRealmClass())
	output.KV(writer, "phase", status.GetPhase())
	output.KV(writer, "readiness", status.GetReadiness())
	output.KV(writer, "reason", status.GetReason())
	output.KV(writer, "authority_epoch", fmt.Sprint(status.GetAuthorityEpoch()))
	output.KV(writer, "authority_sequence", fmt.Sprint(status.GetAuthoritySequence()))
	output.KV(writer, "checkpoint_digest", status.GetCheckpointDigest())
	output.KV(writer, "member_count", fmt.Sprint(status.GetMemberCount()))
	output.KV(writer, "channel_count", fmt.Sprint(status.GetChannelCount()))
	output.KV(writer, "pending_operation_count", fmt.Sprint(status.GetPendingOperationCount()))
	output.KV(writer, "audit_outbox_depth", fmt.Sprint(status.GetAuditOutboxDepth()))
	output.KV(writer, "current_generation", fmt.Sprint(status.GetCurrentGeneration()))
	deadline := ""
	if status.GetOperationDeadline() != nil {
		deadline = status.GetOperationDeadline().AsTime().UTC().Format(time.RFC3339)
	}
	output.KV(writer, "operation_deadline", deadline)
}
