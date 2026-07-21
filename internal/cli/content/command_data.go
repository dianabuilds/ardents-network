// Package content owns content command parsing and presentation.
// It does not own content lifecycle decisions.
package content

import (
	"context"
	"fmt"
	"io"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (a *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderDataUsage(a.ctx.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "inventory":
		return a.dataInventory(ctx)
	case "objects":
		return a.dataObjects(ctx, args[1:])
	case "blobs":
		return a.dataBlobs(ctx, args[1:])
	case "manifests":
		return a.dataManifests(ctx, args[1:])
	case "transfers":
		return a.dataTransfers(ctx, args[1:])
	default:
		output.Writef(a.ctx.Renderer.Err, "ardentsctl data: unknown subcommand %q\n", args[0])
		renderDataUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func renderDataUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] data <inventory|objects|blobs|manifests|transfers>")
}

func requireValue(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func (a *Command) dataInventory(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetDataInventory(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetDataInventoryRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data inventory")
	inv := resp.Msg
	output.KV(a.ctx.Renderer.Out, "objects", fmt.Sprintf("%d", inv.GetObjects()))
	output.KV(a.ctx.Renderer.Out, "manifests", fmt.Sprintf("%d", inv.GetManifests()))
	output.KV(a.ctx.Renderer.Out, "blobs", fmt.Sprintf("%d", inv.GetBlobs()))
	output.KV(a.ctx.Renderer.Out, "local_blobs", fmt.Sprintf("%d", inv.GetLocalBlobs()))
	output.KV(a.ctx.Renderer.Out, "remote_blobs", fmt.Sprintf("%d", inv.GetRemoteBlobs()))
	output.KV(a.ctx.Renderer.Out, "pinned", fmt.Sprintf("%d", inv.GetPinned()))
	output.KV(a.ctx.Renderer.Out, "encrypted", fmt.Sprintf("%d", inv.GetEncrypted()))
	return 0
}
