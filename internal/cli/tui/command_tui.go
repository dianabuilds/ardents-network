// Package tui owns interactive terminal workflow and state.
// It does not own product state or RPC behaviour.
package tui

import (
	"context"
	"io"

	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"

	tea "charm.land/bubbletea/v2"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (a *Command) Run(ctx context.Context, args []string) int {
	if len(args) > 0 {
		if args[0] == "help" {
			renderTUIUsage(a.ctx.Renderer.Out)
			return 0
		}
		output.Writef(a.ctx.Renderer.Err, "ard tui: unknown argument %q\n", args[0])
		renderTUIUsage(a.ctx.Renderer.Err)
		return 2
	}
	program := tea.NewProgram(newTUIModel(ctx, a), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return a.ctx.Failure(err)
	}
	return 0
}

func renderTUIUsage(writer io.Writer) { output.Writeln(writer, "Usage: ard [global flags] tui") }
