package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *app) dataBlobs(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.fail(fmt.Errorf("data blobs subcommand is required"))
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
		return a.fail(fmt.Errorf("unknown data blobs subcommand %q", args[0]))
	}
}

func (a *app) dataBlobsList(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListBlobs(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListBlobsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blobs")
	for _, item := range resp.Msg.GetBlobs() {
		printBlobSummary(a.stdout, item)
	}
	return 0
}

func (a *app) dataBlobsGet(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetBlobRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blob")
	printBlobSummary(a.stdout, resp.Msg)
	return 0
}

func (a *app) dataBlobsPublish(ctx context.Context, args []string) int {
	file, err := parseFileArg("data blobs publish", a.stderr, args)
	if err != nil {
		return a.fail(err)
	}
	blob := &ardentsv1.BlobSnapshot{}
	if err := loadProtoJSON(file, blob); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().PublishBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.PublishBlobRequest{Blob: blob}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blob publish")
	printBlobSummary(a.stdout, resp.Msg)
	return 0
}

func (a *app) dataBlobsFetch(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().FetchBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.FetchBlobRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blob fetch")
	printBlobSummary(a.stdout, resp.Msg)
	return 0
}

func (a *app) dataBlobsSources(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListBlobSources(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListBlobSourcesRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blob sources")
	for _, item := range resp.Msg.GetSources() {
		printKV(a.stdout, "source", item.GetNodeId())
		printKV(a.stdout, "  service", item.GetServiceId())
		printKV(a.stdout, "  usable", boolString(item.GetUsable()))
		printKV(a.stdout, "  reason", item.GetReason())
	}
	return 0
}

func (a *app) dataBlobRetain(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("data blobs retain", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var id string
	var expiresAt string
	fs.StringVar(&id, "id", "", "blob id")
	fs.StringVar(&expiresAt, "expires-at", "", "RFC3339 retention expiry")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if err := requireValue("id", id); err != nil {
		return a.fail(err)
	}
	if err := requireValue("expires-at", expiresAt); err != nil {
		return a.fail(err)
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return a.fail(fmt.Errorf("parse expires-at: %w", err))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().RetainBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.RetainBlobRequest{
		Id:        id,
		ExpiresAt: timestamppb.New(parsed),
	}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "data blob retain")
	printBlobSummary(a.stdout, resp.Msg)
	return 0
}

func (a *app) dataBlobAck(ctx context.Context, action string, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("blob id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	switch action {
	case "pin":
		resp, err := a.client.Service().PinBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.PinBlobRequest{Id: id}))
		if err != nil {
			return a.fail(err)
		}
		if a.jsonMode() {
			renderJSON(a.stdout, resp.Msg)
			return 0
		}
		printHeader(a.stdout, "data blob pin")
		printBlobSummary(a.stdout, resp.Msg)
		return 0
	case "drop":
		resp, err := a.client.Service().DropBlob(callCtx, client.Request(a.cfg.Token, &ardentsv1.DropBlobRequest{Id: id}))
		if err != nil {
			return a.fail(err)
		}
		if a.jsonMode() {
			renderJSON(a.stdout, resp.Msg)
			return 0
		}
		printHeader(a.stdout, "data blob drop")
		printBlobSummary(a.stdout, resp.Msg)
		return 0
	default:
		return a.fail(fmt.Errorf("unsupported blob action %q", action))
	}
}

func printBlobSummary(w io.Writer, item *ardentsv1.BlobSnapshot) {
	if item == nil {
		return
	}
	printKV(w, "blob", item.GetId())
	printKV(w, "  cid", item.GetCid())
	printKV(w, "  media_type", item.GetMediaType())
	printKV(w, "  state", item.GetState())
	printKV(w, "  retention", item.GetRetention())
	printKV(w, "  encrypted", boolString(item.GetEncrypted()))
}
