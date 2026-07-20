package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

type app struct {
	cfg    Config
	stdin  io.Reader
	client *client.Client
	stdout io.Writer
	stderr io.Writer
}

func newApp(cfg Config, stdin io.Reader, stdout, stderr io.Writer) *app {
	return &app{
		cfg:   cfg,
		stdin: stdin,
		client: client.New(client.Config{
			BaseURL:           cfg.Addr,
			Token:             cfg.Token,
			Timeout:           cfg.Timeout,
			ExpectedNode:      cfg.ExpectedNode,
			ExpectedPrincipal: cfg.ExpectedPrincipal,
			Scopes:            cfg.ScopeHints,
		}),
		stdout: stdout,
		stderr: stderr,
	}
}

func (a *app) commandContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, a.cfg.Timeout)
}

func (a *app) jsonMode() bool {
	return a.cfg.Output == "json"
}

func (a *app) fail(err error) int {
	renderError(a.stderr, a.jsonMode(), err)
	return 1
}

func requireValue(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func (a *app) verifyIdentity(ctx context.Context) error {
	if !a.cfg.hasIdentityBinding() {
		return nil
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNodeRuntime(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeRuntimeRequest{}))
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
