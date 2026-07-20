package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) runData(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderDataUsage(a.stdout)
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
		_, _ = fmt.Fprintf(a.stderr, "ard data: unknown subcommand %q\n", args[0])
		renderDataUsage(a.stderr)
		return 2
	}
}

func (a *app) dataInventory(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetDataInventory(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetDataInventoryRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data inventory")
	inv := resp.Msg
	printKV(a.stdout, "objects", fmt.Sprintf("%d", inv.GetObjects()))
	printKV(a.stdout, "manifests", fmt.Sprintf("%d", inv.GetManifests()))
	printKV(a.stdout, "blobs", fmt.Sprintf("%d", inv.GetBlobs()))
	printKV(a.stdout, "local_blobs", fmt.Sprintf("%d", inv.GetLocalBlobs()))
	printKV(a.stdout, "remote_blobs", fmt.Sprintf("%d", inv.GetRemoteBlobs()))
	printKV(a.stdout, "pinned", fmt.Sprintf("%d", inv.GetPinned()))
	printKV(a.stdout, "encrypted", fmt.Sprintf("%d", inv.GetEncrypted()))
	return 0
}
