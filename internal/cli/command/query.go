package command

import (
	"context"
	"io"

	"ardents/internal/cli/output"

	"google.golang.org/protobuf/proto"
)

// RunQuery owns the common CLI query lifecycle: watch execution, timeout,
// failure handling, and JSON-versus-human rendering.
func RunQuery[T proto.Message](ctx context.Context, command Context, session string,
	fetch func(context.Context) (T, error), renderHuman func(io.Writer, T),
) int {
	if command.Watch {
		return command.WatchRun(ctx, session, func(callCtx context.Context) (proto.Message, error) {
			return fetch(callCtx)
		}, func(writer io.Writer, message proto.Message) {
			renderHuman(writer, message.(T))
		})
	}
	return RunOnce(ctx, command, fetch, renderHuman)
}

// RunOnce owns one timeout-bound CLI query and its output mode.
func RunOnce[T proto.Message](ctx context.Context, command Context,
	fetch func(context.Context) (T, error), renderHuman func(io.Writer, T),
) int {
	callCtx, cancel := command.Call(ctx)
	defer cancel()
	message, err := fetch(callCtx)
	if err != nil {
		return command.Failure(err)
	}
	if command.Renderer.JSON {
		output.JSON(command.Renderer.Out, message)
		return 0
	}
	renderHuman(command.Renderer.Out, message)
	return 0
}
