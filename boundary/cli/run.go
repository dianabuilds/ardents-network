package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWithIO(ctx, args, os.Stdin, stdout, stderr)
}

func runWithIO(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, rest, help, err := parseRoot(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "ard: %v\n", err)
		return 2
	}
	if help || len(rest) == 0 {
		renderRootUsage(stdout)
		return 0
	}
	return dispatch(ctx, cfg, rest, stdin, stdout, stderr)
}

func dispatch(ctx context.Context, cfg Config, rest []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if code, ok := renderGroupIfRequested(rest, stdout); ok {
		return code
	}
	if rest[0] == "help" {
		renderRootUsage(stdout)
		return 0
	}
	if rest[0] == "version" {
		return renderVersion(stdout, cfg.Output)
	}
	if err := cfg.Resolve(); err != nil {
		_, _ = fmt.Fprintf(stderr, "ard: %v\n", err)
		return 2
	}
	app := newApp(cfg, stdin, stdout, stderr)
	if err := app.verifyIdentity(ctx); err != nil {
		return app.fail(err)
	}
	return app.dispatch(ctx, rest)
}

func (a *app) dispatch(ctx context.Context, rest []string) int {
	switch rest[0] {
	case "node":
		return a.runNode(ctx, rest[1:])
	case "network":
		return a.runNetwork(ctx, rest[1:])
	case "diagnostics":
		return a.runDiagnostics(ctx, rest[1:])
	case "config":
		return a.runConfig(ctx, rest[1:])
	case "workload":
		return a.runWorkload(ctx, rest[1:])
	case "data":
		return a.runData(ctx, rest[1:])
	case "tui":
		return a.runTUI(ctx, rest[1:])
	case "shell":
		return a.runShell(ctx, rest[1:])
	default:
		_, _ = fmt.Fprintf(a.stderr, "ard: unknown command %q\n", rest[0])
		renderRootUsage(a.stderr)
		return 2
	}
}

func renderGroupIfRequested(rest []string, stdout io.Writer) (int, bool) {
	if len(rest) == 0 {
		renderRootUsage(stdout)
		return 0, true
	}
	if !isGroup(rest[0]) {
		return 0, false
	}
	if len(rest) > 1 && rest[1] != "help" {
		return 0, false
	}
	renderGroupUsage(stdout, rest[0])
	return 0, true
}

func isGroup(name string) bool {
	switch name {
	case "node", "network", "workload", "data", "diagnostics", "config", "tui":
		return true
	default:
		return false
	}
}

func parseRoot(args []string, stderr io.Writer) (Config, []string, bool, error) {
	cfg := defaultConfig()
	fs := flag.NewFlagSet("ard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var scopeHints multiString
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "local API address")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "bearer token override")
	fs.StringVar(&cfg.TokenFile, "token-file", cfg.TokenFile, "path to bearer token file")
	fs.StringVar(&cfg.ContextName, "context", cfg.ContextName, "named operator context")
	fs.StringVar(&cfg.ContextFile, "context-file", cfg.ContextFile, "path to contexts file")
	fs.StringVar(&cfg.ExpectedNode, "node-name", cfg.ExpectedNode, "expected node name for identity preflight")
	fs.StringVar(&cfg.ExpectedPrincipal, "principal", cfg.ExpectedPrincipal, "expected node principal for identity preflight")
	fs.StringVar(&cfg.ExpectedPublicKey, "public-key", cfg.ExpectedPublicKey, "expected node public key for identity preflight")
	fs.Var(&scopeHints, "scope", "exact action scope that narrows this operator context")
	fs.StringVar(&cfg.Output, "output", cfg.Output, "output mode: human or json")
	fs.BoolVar(&cfg.Watch, "watch", cfg.Watch, "watch mode for commands that support live updates")
	fs.DurationVar(&cfg.Interval, "interval", cfg.Interval, "poll interval for watch mode")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "request timeout")
	var help bool
	fs.BoolVar(&help, "help", false, "show help")
	fs.BoolVar(&help, "h", false, "show help")
	if err := fs.Parse(args); err != nil {
		return Config{}, nil, false, err
	}
	cfg.ScopeHints = scopeHints
	return cfg, fs.Args(), help, nil
}

type multiString []string

func (m *multiString) String() string { return "" }

func (m *multiString) Set(value string) error {
	if value == "" {
		return fmt.Errorf("scope value must not be empty")
	}
	*m = append(*m, value)
	return nil
}
