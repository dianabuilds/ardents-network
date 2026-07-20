package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

func (a *app) runShell(ctx context.Context, args []string) int {
	if code, stop := a.validateShellStart(args); stop {
		return code
	}
	a.printShellBanner()
	scanner := bufio.NewScanner(a.stdin)
	for {
		if _, err := fmt.Fprint(a.stdout, "ard> "); err != nil {
			return a.fail(err)
		}
		line, done, err := scanShellLine(scanner)
		if err != nil {
			return a.fail(err)
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
		if code := a.dispatch(ctx, strings.Fields(line)); code != 0 {
			return code
		}
	}
}

func (a *app) validateShellStart(args []string) (int, bool) {
	if len(args) > 0 {
		if args[0] == "help" {
			renderShellUsage(a.stdout)
			return 0, true
		}
		_, _ = fmt.Fprintf(a.stderr, "ard shell: unknown argument %q\n", args[0])
		renderShellUsage(a.stderr)
		return 2, true
	}
	if a.jsonMode() {
		return a.fail(fmt.Errorf("shell does not support --output=json")), true
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

func (a *app) printShellBanner() {
	_, _ = fmt.Fprintln(a.stdout, "interactive shell")
	a.renderShellContext()
	_, _ = fmt.Fprintln(a.stdout, "type 'help' for shell commands, 'exit' to leave")
}

func (a *app) renderShellHelp() {
	_, _ = fmt.Fprintln(a.stdout, "Shell commands:")
	_, _ = fmt.Fprintln(a.stdout, "  context   show current operator context and identity binding")
	_, _ = fmt.Fprintln(a.stdout, "  help      show shell help")
	_, _ = fmt.Fprintln(a.stdout, "  exit      leave the shell")
	_, _ = fmt.Fprintln(a.stdout, "  quit      leave the shell")
	_, _ = fmt.Fprintln(a.stdout)
	renderRootUsage(a.stdout)
}

func (a *app) renderShellContext() {
	printHeader(a.stdout, "current context")
	printKV(a.stdout, "addr", a.cfg.Addr)
	printKV(a.stdout, "context", valueOrDash(a.cfg.ContextName))
	printKV(a.stdout, "principal", valueOrDash(a.cfg.ExpectedPrincipal))
	printKV(a.stdout, "node", valueOrDash(a.cfg.ExpectedNode))
	printKV(a.stdout, "public_key", valueOrDash(a.cfg.ExpectedPublicKey))
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
