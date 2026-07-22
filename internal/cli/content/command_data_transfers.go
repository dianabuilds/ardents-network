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

func (a *Command) dataTransfers(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return a.ctx.Failure(fmt.Errorf("data transfers subcommand is required"))
	}
	switch args[0] {
	case "list":
		return a.dataTransfersList(ctx)
	case "get":
		return a.dataTransfersGet(ctx, args[1:])
	default:
		return a.ctx.Failure(fmt.Errorf("unknown data transfers subcommand %q", args[0]))
	}
}

func (a *Command) dataTransfersList(ctx context.Context) int {
	return commandctx.RunQuery(ctx, a.ctx, "data transfers", func(callCtx context.Context) (*ardentsv1.ListTransfersResponse, error) {
		resp, err := a.ctx.Client.Service().ListTransfers(callCtx, client.Request(&ardentsv1.ListTransfersRequest{}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}, renderDataTransfersListHuman)
}

func (a *Command) dataTransfersGet(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("transfer id is required"))
	}
	return commandctx.RunQuery(ctx, a.ctx, "data transfer", func(callCtx context.Context) (*ardentsv1.GetTransferResponse, error) {
		resp, err := a.ctx.Client.Service().GetTransfer(callCtx, client.Request(&ardentsv1.GetTransferRequest{Id: id}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}, renderDataTransferHuman)
}

func renderDataTransfersListHuman(w io.Writer, msg *ardentsv1.ListTransfersResponse) {
	output.Header(w, "data transfers")
	if len(msg.GetTransfers()) == 0 {
		output.Writeln(w, "no transfers")
		return
	}
	for _, item := range msg.GetTransfers() {
		printTransferSummary(w, item)
	}
}

func renderDataTransferHuman(w io.Writer, msg *ardentsv1.GetTransferResponse) {
	output.Header(w, "data transfer")
	output.Status(w, msg.GetStatus())
	printTransferSummary(w, msg.GetTransfer())
}

func printTransferSummary(w io.Writer, item *ardentsv1.TransferSnapshot) {
	if item == nil {
		return
	}
	output.KV(w, "transfer", item.GetId())
	output.KV(w, "  kind", item.GetKind())
	output.KV(w, "  state", item.GetState())
	output.KV(w, "  resource", item.GetResourceId())
	output.KV(w, "  reason", item.GetReason())
}
