package cli

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (a *app) runTUI(ctx context.Context, args []string) int {
	if len(args) > 0 {
		if args[0] == "help" {
			renderTUIUsage(a.stdout)
			return 0
		}
		_, _ = fmt.Fprintf(a.stderr, "ard tui: unknown argument %q\n", args[0])
		renderTUIUsage(a.stderr)
		return 2
	}
	program := tea.NewProgram(newTUIModel(ctx, a), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return a.fail(err)
	}
	return 0
}
