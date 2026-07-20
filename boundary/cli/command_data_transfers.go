package cli

import (
	"context"
	"fmt"
	"io"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/proto"
)

func (a *app) dataTransfers(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.fail(fmt.Errorf("data transfers subcommand is required"))
	}
	switch args[0] {
	case "list":
		return a.dataTransfersList(ctx)
	case "get":
		return a.dataTransfersGet(ctx, args[1:])
	default:
		return a.fail(fmt.Errorf("unknown data transfers subcommand %q", args[0]))
	}
}

func (a *app) dataTransfersList(ctx context.Context) int {
	if a.cfg.Watch {
		return a.watchSnapshots(ctx, "data transfers", func(callCtx context.Context) (proto.Message, error) {
			resp, err := a.client.Service().ListTransfers(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListTransfersRequest{}))
			if err != nil {
				return nil, err
			}
			return resp.Msg, nil
		}, func(w io.Writer, msg proto.Message) {
			renderDataTransfersListHuman(w, msg.(*ardentsv1.ListTransfersResponse))
		})
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListTransfers(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListTransfersRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderDataTransfersListHuman(a.stdout, resp.Msg)
	return 0
}

func (a *app) dataTransfersGet(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("transfer id is required"))
	}
	if a.cfg.Watch {
		return a.watchSnapshots(ctx, "data transfer", func(callCtx context.Context) (proto.Message, error) {
			resp, err := a.client.Service().GetTransfer(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetTransferRequest{Id: id}))
			if err != nil {
				return nil, err
			}
			return resp.Msg, nil
		}, func(w io.Writer, msg proto.Message) {
			renderDataTransferHuman(w, msg.(*ardentsv1.GetTransferResponse))
		})
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetTransfer(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetTransferRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderDataTransferHuman(a.stdout, resp.Msg)
	return 0
}

func renderDataTransfersListHuman(w io.Writer, msg *ardentsv1.ListTransfersResponse) {
	printHeader(w, "data transfers")
	if len(msg.GetTransfers()) == 0 {
		_, _ = fmt.Fprintln(w, "no transfers")
		return
	}
	for _, item := range msg.GetTransfers() {
		printTransferSummary(w, item)
	}
}

func renderDataTransferHuman(w io.Writer, msg *ardentsv1.GetTransferResponse) {
	printHeader(w, "data transfer")
	printStatusLine(w, msg.GetStatus())
	printTransferSummary(w, msg.GetTransfer())
}

func printTransferSummary(w io.Writer, item *ardentsv1.TransferSnapshot) {
	if item == nil {
		return
	}
	printKV(w, "transfer", item.GetId())
	printKV(w, "  kind", item.GetKind())
	printKV(w, "  state", item.GetState())
	printKV(w, "  resource", item.GetResourceId())
	printKV(w, "  reason", item.GetReason())
}
