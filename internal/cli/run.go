package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	configurationcmd "ardents/internal/cli/configuration"
	contentcmd "ardents/internal/cli/content"
	diagnosticscmd "ardents/internal/cli/diagnostics"
	identitycmd "ardents/internal/cli/identity"
	networkcmd "ardents/internal/cli/network"
	nodecmd "ardents/internal/cli/node"
	"ardents/internal/cli/output"
	tuicmd "ardents/internal/cli/tui"
	workloadcmd "ardents/internal/cli/workload"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWithIO(ctx, args, os.Stdin, stdout, stderr)
}

func runWithIO(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	renderer := output.NewRenderer(stdout, stderr, false)
	cfg, rest, help, err := parseRoot(args, renderer.Err)
	if err != nil {
		output.Writef(renderer.Err, "ardentsctl: %v\n", err)
		return 2
	}
	if help || len(rest) == 0 {
		renderRootUsage(renderer.Out)
		if renderer.OutputError() != nil {
			return 1
		}
		return 0
	}
	code := dispatch(ctx, cfg, rest, stdin, renderer.Out, renderer.Err)
	if renderer.OutputError() != nil && code == 0 {
		return 1
	}
	return code
}

func dispatch(ctx context.Context, cfg configurationcmd.Config, rest []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	if rest[0] == "identity" && (len(rest) == 1 || rest[1] == "help" || rest[1] == "principal" || rest[1] == "device" && (len(rest) < 3 || rest[2] != "revoke")) {
		if cfg.Output != "human" && cfg.Output != "json" {
			output.Writef(stderr, "ardentsctl: unsupported output %q\n", cfg.Output)
			return 2
		}
		renderer := output.NewRenderer(stdout, stderr, cfg.Output == "json")
		code := identitycmd.New(renderer).Run(ctx, rest[1:])
		if renderer.OutputError() != nil && code == 0 {
			return 1
		}
		return code
	}
	if err := cfg.Resolve(); err != nil {
		output.Writef(stderr, "ardentsctl: %v\n", err)
		return 2
	}
	app, err := newApp(cfg, stdin, stdout, stderr)
	if err != nil {
		return output.NewRenderer(stdout, stderr, cfg.Output == "json").Failure(err)
	}
	defer app.client.Close()
	if rest[0] == "identity" {
		return app.dispatch(ctx, rest)
	}
	if err := app.verifyIdentity(ctx); err != nil {
		return app.fail(err)
	}
	return app.dispatch(ctx, rest)
}

func (a *app) dispatch(ctx context.Context, rest []string) int {
	var code int
	switch rest[0] {
	case "node":
		code = nodecmd.New(a.command()).Run(ctx, rest[1:])
	case "network":
		code = networkcmd.New(a.command()).Run(ctx, rest[1:])
	case "diagnostics":
		code = diagnosticscmd.New(a.command()).Run(ctx, rest[1:])
	case "config":
		code = configurationcmd.FromContext(a.command()).Run(ctx, rest[1:])
	case "workload":
		code = workloadcmd.New(a.command()).Run(ctx, rest[1:])
	case "data":
		code = contentcmd.New(a.command()).Run(ctx, rest[1:])
	case "identity":
		code = identitycmd.NewOnline(a.renderer, a.client, a.cfg.Timeout, a.stdin).Run(ctx, rest[1:])
	case "tui":
		code = tuicmd.New(a.command()).Run(ctx, rest[1:])
	case "shell":
		code = tuicmd.NewShell(a.command()).Run(ctx, rest[1:])
	default:
		output.Writef(a.renderer.Err, "ardentsctl: unknown command %q\n", rest[0])
		renderRootUsage(a.renderer.Err)
		code = 2
	}
	if a.renderer.OutputError() != nil && code == 0 {
		return 1
	}
	return code
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
	case "node", "network", "workload", "data", "diagnostics", "config", "identity", "tui":
		return true
	default:
		return false
	}
}

func parseRoot(args []string, stderr io.Writer) (configurationcmd.Config, []string, bool, error) {
	cfg := configurationcmd.DefaultConfig()
	fs := flag.NewFlagSet("ardentsctl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var scopeHints multiString
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "local API address")
	fs.StringVar(&cfg.SSH, "ssh", cfg.SSH, "OpenSSH target for secure remote access")
	fs.IntVar(&cfg.SSHPort, "ssh-port", cfg.SSHPort, "OpenSSH server port")
	fs.StringVar(&cfg.SSHIdentity, "ssh-identity", cfg.SSHIdentity, "OpenSSH private key path")
	fs.StringVar(&cfg.SSHKnownHosts, "ssh-known-hosts", cfg.SSHKnownHosts, "OpenSSH known_hosts path")
	fs.StringVar(&cfg.SSHOperatorSocket, "ssh-operator-socket", cfg.SSHOperatorSocket, "absolute remote Operator Unix socket")
	fs.StringVar(&cfg.SignerFile, "signer-file", cfg.SignerFile, "protected device signer bundle")
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
		return configurationcmd.Config{}, nil, false, err
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
