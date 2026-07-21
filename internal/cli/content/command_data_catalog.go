package content

import (
	"context"
	"fmt"
	"io"

	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type catalogSnapshot interface {
	proto.Message
	GetId() string
	GetOwner() string
}

type catalogResource[S catalogSnapshot, L proto.Message] struct {
	command        *Command
	singular       string
	plural         string
	attribute      string
	newSnapshot    func() S
	attributeValue func(S) string
	list           func(context.Context) (L, error)
	items          func(L) []S
	get            func(context.Context, string) (S, error)
	publish        func(context.Context, S) (S, error)
}

func connectMessage[T any](response *connect.Response[T], err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	return response.Msg, nil
}

func (r catalogResource[S, L]) run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return r.command.ctx.Failure(fmt.Errorf("data %s subcommand is required", r.plural))
	}
	switch args[0] {
	case "list":
		return r.runList(ctx)
	case "get":
		return r.runGet(ctx, args[1:])
	case "publish":
		return r.runPublish(ctx, args[1:])
	default:
		return r.command.ctx.Failure(fmt.Errorf("unknown data %s subcommand %q", r.plural, args[0]))
	}
}

func (r catalogResource[S, L]) runList(ctx context.Context) int {
	return commandctx.RunOnce(ctx, r.command.ctx, r.list, func(writer io.Writer, response L) {
		output.Header(writer, "data "+r.plural)
		for _, item := range r.items(response) {
			output.KV(writer, r.singular, item.GetId())
			output.KV(writer, "  "+r.attribute, r.attributeValue(item))
			output.KV(writer, "  owner", item.GetOwner())
		}
	})
}

func (r catalogResource[S, L]) runGet(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return r.command.ctx.Failure(fmt.Errorf("%s id is required", r.singular))
	}
	return commandctx.RunOnce(ctx, r.command.ctx, func(callCtx context.Context) (S, error) {
		return r.get(callCtx, id)
	}, func(writer io.Writer, item S) {
		output.Header(writer, "data "+r.singular)
		output.KV(writer, "id", item.GetId())
		output.KV(writer, r.attribute, r.attributeValue(item))
		output.KV(writer, "owner", item.GetOwner())
	})
}

func (r catalogResource[S, L]) runPublish(ctx context.Context, args []string) int {
	file, err := commandctx.ParseFileArg("data "+r.plural+" publish", r.command.ctx.Renderer.Err, args)
	if err != nil {
		return r.command.ctx.Failure(err)
	}
	item := r.newSnapshot()
	if err := commandctx.LoadProtoJSON(r.command.ctx.Input, file, item); err != nil {
		return r.command.ctx.Failure(err)
	}
	return commandctx.RunOnce(ctx, r.command.ctx, func(callCtx context.Context) (S, error) {
		return r.publish(callCtx, item)
	}, func(writer io.Writer, published S) {
		output.Header(writer, "data "+r.singular+" publish")
		output.KV(writer, "id", published.GetId())
		output.KV(writer, r.attribute, r.attributeValue(published))
	})
}
