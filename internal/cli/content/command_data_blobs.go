package content

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *Command) dataBlobs(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.ctx.Failure(fmt.Errorf("data blobs subcommand is required"))
	}
	switch args[0] {
	case "list":
		return a.dataBlobsList(ctx)
	case "get":
		return a.dataBlobsGet(ctx, args[1:])
	case "publish":
		return a.dataBlobsPublish(ctx, args[1:])
	case "fetch":
		return a.dataBlobsFetch(ctx, args[1:])
	case "sources":
		return a.dataBlobsSources(ctx, args[1:])
	case "retain":
		return a.dataBlobRetain(ctx, args[1:])
	case "pin":
		return a.dataBlobAck(ctx, "pin", args[1:])
	case "drop":
		return a.dataBlobAck(ctx, "drop", args[1:])
	default:
		return a.ctx.Failure(fmt.Errorf("unknown data blobs subcommand %q", args[0]))
	}
}

func (a *Command) dataBlobsList(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListBlobs(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListBlobsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blobs")
	for _, item := range resp.Msg.GetBlobs() {
		printBlobSummary(a.ctx.Renderer.Out, item)
	}
	return 0
}

func (a *Command) dataBlobsGet(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetBlobRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blob")
	printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) dataBlobsPublish(ctx context.Context, args []string) int {
	file, err := commandctx.ParseFileArg("data blobs publish", a.ctx.Renderer.Err, args)
	if err != nil {
		return a.ctx.Failure(err)
	}
	blob := &ardentsv1.BlobSnapshot{}
	if err := commandctx.LoadProtoJSON(a.ctx.Input, file, blob); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().PublishBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.PublishBlobRequest{Blob: blob}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blob publish")
	printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) dataBlobsFetch(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().FetchBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.FetchBlobRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blob fetch")
	printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) dataBlobsSources(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListBlobSources(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListBlobSourcesRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blob sources")
	for _, item := range resp.Msg.GetSources() {
		output.KV(a.ctx.Renderer.Out, "source", item.GetNodeId())
		output.KV(a.ctx.Renderer.Out, "  service", item.GetServiceId())
		output.KV(a.ctx.Renderer.Out, "  usable", output.Bool(item.GetUsable()))
		output.KV(a.ctx.Renderer.Out, "  reason", item.GetReason())
	}
	return 0
}

func (a *Command) dataBlobRetain(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("data blobs retain", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var id string
	var expiresAt string
	fs.StringVar(&id, "id", "", "blob id")
	fs.StringVar(&expiresAt, "expires-at", "", "RFC3339 retention expiry")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("id", id); err != nil {
		return a.ctx.Failure(err)
	}
	if err := requireValue("expires-at", expiresAt); err != nil {
		return a.ctx.Failure(err)
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return a.ctx.Failure(fmt.Errorf("parse expires-at: %w", err))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().RetainBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.RetainBlobRequest{
		Id:        id,
		ExpiresAt: timestamppb.New(parsed),
	}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "data blob retain")
	printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) dataBlobAck(ctx context.Context, action string, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	switch action {
	case "pin":
		resp, err := a.ctx.Client.Service().PinBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.PinBlobRequest{Id: id}))
		if err != nil {
			return a.ctx.Failure(err)
		}
		if a.ctx.Renderer.JSON {
			output.JSON(a.ctx.Renderer.Out, resp.Msg)
			return 0
		}
		output.Header(a.ctx.Renderer.Out, "data blob pin")
		printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
		return 0
	case "drop":
		resp, err := a.ctx.Client.Service().DropBlob(callCtx, client.Request(a.ctx.Token, &ardentsv1.DropBlobRequest{Id: id}))
		if err != nil {
			return a.ctx.Failure(err)
		}
		if a.ctx.Renderer.JSON {
			output.JSON(a.ctx.Renderer.Out, resp.Msg)
			return 0
		}
		output.Header(a.ctx.Renderer.Out, "data blob drop")
		printBlobSummary(a.ctx.Renderer.Out, resp.Msg)
		return 0
	default:
		return a.ctx.Failure(fmt.Errorf("unsupported blob action %q", action))
	}
}

func printBlobSummary(w io.Writer, item *ardentsv1.BlobSnapshot) {
	if item == nil {
		return
	}
	output.KV(w, "blob", item.GetId())
	output.KV(w, "  cid", item.GetCid())
	output.KV(w, "  media_type", item.GetMediaType())
	output.KV(w, "  state", item.GetState())
	output.KV(w, "  retention", item.GetRetention())
	output.KV(w, "  encrypted", output.Bool(item.GetEncrypted()))
}
