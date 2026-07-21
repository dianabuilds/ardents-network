package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
)

type Shell struct{ ctx commandctx.Context }

func NewShell(ctx commandctx.Context) *Shell { return &Shell{ctx: ctx} }

func (a *Shell) Run(ctx context.Context, args []string) int {
	if code, stop := a.validateShellStart(args); stop {
		return code
	}
	a.printShellBanner()
	scanner := bufio.NewScanner(a.ctx.Input)
	for {
		if _, err := fmt.Fprint(a.ctx.Renderer.Out, "ardents> "); err != nil {
			return a.ctx.Failure(err)
		}
		line, done, err := scanShellLine(scanner)
		if err != nil {
			return a.ctx.Failure(err)
		}
		if done {
			return 0
		}
		if line == "" {
			continue
		}
		switch line {
		case "exit", "quit":
			return 0
		case "help":
			a.renderShellHelp()
			continue
		case "context":
			a.renderShellContext()
			continue
		}
		if code := a.ctx.Dispatch(ctx, strings.Fields(line)); code != 0 {
			return code
		}
	}
}

func renderShellUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] shell")
}

func (a *Shell) validateShellStart(args []string) (int, bool) {
	if len(args) > 0 {
		if args[0] == "help" {
			renderShellUsage(a.ctx.Renderer.Out)
			return 0, true
		}
		output.Writef(a.ctx.Renderer.Err, "ardentsctl shell: unknown argument %q\n", args[0])
		renderShellUsage(a.ctx.Renderer.Err)
		return 2, true
	}
	if a.ctx.Renderer.JSON {
		return a.ctx.Failure(fmt.Errorf("shell does not support --output=json")), true
	}
	return 0, false
}

func scanShellLine(scanner *bufio.Scanner) (string, bool, error) {
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), false, nil
	}
	if err := scanner.Err(); err != nil {
		return "", false, fmt.Errorf("read shell input: %w", err)
	}
	return "", true, nil
}

func (a *Shell) printShellBanner() {
	output.Writeln(a.ctx.Renderer.Out, "interactive shell")
	a.renderShellContext()
	output.Writeln(a.ctx.Renderer.Out, "type 'help' for shell commands, 'exit' to leave")
}

func (a *Shell) renderShellHelp() {
	output.Writeln(a.ctx.Renderer.Out, "Shell commands:")
	output.Writeln(a.ctx.Renderer.Out, "  context   show current operator context and identity binding")
	output.Writeln(a.ctx.Renderer.Out, "  help      show shell help")
	output.Writeln(a.ctx.Renderer.Out, "  exit      leave the shell")
	output.Writeln(a.ctx.Renderer.Out, "  quit      leave the shell")
	output.Writeln(a.ctx.Renderer.Out)
	a.ctx.Usage(a.ctx.Renderer.Out)
}

func (a *Shell) renderShellContext() {
	output.Header(a.ctx.Renderer.Out, "current context")
	output.KV(a.ctx.Renderer.Out, "addr", a.ctx.Operator.Address)
	output.KV(a.ctx.Renderer.Out, "context", valueOrDash(a.ctx.Operator.Name))
	output.KV(a.ctx.Renderer.Out, "principal", valueOrDash(a.ctx.Operator.Principal))
	output.KV(a.ctx.Renderer.Out, "node", valueOrDash(a.ctx.Operator.Node))
	output.KV(a.ctx.Renderer.Out, "public_key", valueOrDash(a.ctx.Operator.PublicKey))
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
