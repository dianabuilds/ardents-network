// Package configuration owns operator configuration commands and presentation.
// It does not own configuration ownership or runtime application.
package configuration

import (
	"context"
	"fmt"
	"io"
	"time"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	protocol "ardents/internal/localapi/protocol"
)

type Command struct {
	Client   *client.Client
	Token    string
	Timeout  time.Duration
	Renderer output.Renderer
}

func FromContext(ctx commandctx.Context) Command {
	return Command{Client: ctx.Client, Token: ctx.Token, Timeout: ctx.Timeout, Renderer: ctx.Renderer}
}

func (c Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		usage(c.Renderer.Out)
		return 0
	}
	if len(args) != 1 {
		return c.usageError("subcommand does not accept positional arguments")
	}
	switch args[0] {
	case "show":
		return c.show(ctx)
	case "reload":
		return c.reload(ctx)
	default:
		return c.usageError(fmt.Sprintf("unknown subcommand %q", args[0]))
	}
}

func (c Command) show(ctx context.Context) int {
	callCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	response, err := c.Client.Service().GetEffectiveConfiguration(callCtx, client.Request(c.Token, &protocol.GetEffectiveConfigurationRequest{}))
	if err != nil {
		return c.Renderer.Failure(err)
	}
	if c.Renderer.JSON {
		c.Renderer.Message(response.Msg)
		return 0
	}
	c.Renderer.Header("effective configuration")
	c.Renderer.Status(response.Msg.GetStatus())
	c.renderSnapshot(response.Msg.GetConfiguration())
	return 0
}

func (c Command) reload(ctx context.Context) int {
	callCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	response, err := c.Client.Service().ReloadConfiguration(callCtx, client.Request(c.Token, &protocol.ReloadConfigurationRequest{}))
	if err != nil {
		return c.Renderer.Failure(err)
	}
	if c.Renderer.JSON {
		c.Renderer.Message(response.Msg)
		return 0
	}
	c.Renderer.Header("configuration reload")
	c.Renderer.Status(response.Msg.GetStatus())
	result := response.Msg.GetResult()
	c.Renderer.KV("outcome", result.GetOutcome())
	c.Renderer.KV("active_generation", fmt.Sprint(result.GetActiveGeneration()))
	c.Renderer.KV("candidate_generation", fmt.Sprint(result.GetCandidateGeneration()))
	c.Renderer.CSV("restart_required", result.GetRestartRequired())
	c.Renderer.CSV("immutable", result.GetImmutable())
	return 0
}

func (c Command) renderSnapshot(snapshot *protocol.EffectiveConfigurationSnapshot) {
	c.Renderer.KV("api_version", snapshot.GetApiVersion())
	c.Renderer.KV("active_generation", fmt.Sprint(snapshot.GetActiveGeneration()))
	c.Renderer.KV("candidate_generation", fmt.Sprint(snapshot.GetCandidateGeneration()))
	c.Renderer.KV("fingerprint", snapshot.GetFingerprint())
	c.Renderer.KV("last_reload", snapshot.GetLastReloadOutcome())
	c.Renderer.CSV("pending_restart", snapshot.GetPendingRestart())
	if snapshot.GetEffective() != nil {
		c.Renderer.Message(snapshot.GetEffective())
	}
}

func (c Command) usageError(message string) int {
	output.Writef(c.Renderer.Err, "ard config: %s\n", message)
	usage(c.Renderer.Err)
	return 2
}

func usage(writer io.Writer) {
	output.Writeln(writer, "Usage: ard [global flags] config <show|reload>")
}
