// Package cli owns root CLI argument dispatch, usage, and version commands.
// It does not own product behaviour or area command implementation.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	configurationcmd "ardents/internal/cli/configuration"
	identitycmd "ardents/internal/cli/identity"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type app struct {
	cfg      configurationcmd.Config
	stdin    io.Reader
	client   *client.Client
	stdout   io.Writer
	stderr   io.Writer
	renderer output.Renderer
}

func (a *app) command() commandctx.Context {
	ctx := commandctx.Context{
		Client: a.client, Input: a.stdin, Timeout: a.cfg.Timeout, Interval: a.cfg.Interval, Watch: a.cfg.Watch,
		Renderer: a.renderer,
		Dispatch: a.dispatch, Usage: renderRootUsage,
		Operator: commandctx.Operator{Address: a.cfg.Addr, Name: a.cfg.ContextName, Principal: a.cfg.ExpectedPrincipal, Node: a.cfg.ExpectedNode, PublicKey: a.cfg.ExpectedPublicKey},
	}
	ctx.WatchRun = ctx.RunWatch
	return ctx
}

func newApp(cfg configurationcmd.Config, stdin io.Reader, stdout, stderr io.Writer) (*app, error) {
	opened, err := identitycmd.OpenDeviceFileSigner(cfg.SignerFile)
	if err != nil {
		return nil, err
	}
	operatorClient, err := client.New(client.Config{
		BaseURL:           cfg.Addr,
		SSH:               cfg.SSH,
		SSHPort:           cfg.SSHPort,
		SSHIdentity:       cfg.SSHIdentity,
		SSHKnownHosts:     cfg.SSHKnownHosts,
		SSHOperatorSocket: cfg.SSHOperatorSocket,
		Timeout:           cfg.Timeout,
		ExpectedNode:      cfg.ExpectedNode,
		ExpectedPrincipal: cfg.ExpectedPrincipal,
		Scopes:            cfg.ScopeHints,
		Signer:            opened,
	})
	if err != nil {
		return nil, err
	}
	return &app{
		cfg:      cfg,
		stdin:    stdin,
		client:   operatorClient,
		stdout:   stdout,
		stderr:   stderr,
		renderer: output.NewRenderer(stdout, stderr, cfg.Output == "json"),
	}, nil
}

func (a *app) commandContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, a.cfg.Timeout)
}

func (a *app) jsonMode() bool {
	return a.cfg.Output == "json"
}

func (a *app) fail(err error) int { return a.renderer.Failure(err) }

func (a *app) verifyIdentity(ctx context.Context) error {
	if !a.cfg.HasIdentityBinding() {
		return nil
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNodeRuntime(callCtx, client.Request(&ardentsv1.GetNodeRuntimeRequest{}))
	if err != nil {
		return fmt.Errorf("identity preflight failed: %w", err)
	}
	identity := resp.Msg.GetRuntime().GetIdentity()
	if expected := strings.TrimSpace(a.cfg.ExpectedNode); expected != "" && resp.Msg.GetRuntime().GetNode().GetName() != expected {
		return fmt.Errorf("identity preflight failed: node mismatch: expected %q, got %q", expected, resp.Msg.GetRuntime().GetNode().GetName())
	}
	if expected := strings.TrimSpace(a.cfg.ExpectedPrincipal); expected != "" && identity.GetPrincipal() != expected {
		return fmt.Errorf("identity preflight failed: principal mismatch: expected %q, got %q", expected, identity.GetPrincipal())
	}
	if expected := strings.TrimSpace(a.cfg.ExpectedPublicKey); expected != "" && identity.GetPublicKey() != expected {
		return fmt.Errorf("identity preflight failed: public key mismatch: expected %q, got %q", expected, identity.GetPublicKey())
	}
	return nil
}
