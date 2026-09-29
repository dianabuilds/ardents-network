package process

import (
	"context"
	"errors"
)

// Run joins work and optional process diagnostics. An empty socket has no
// diagnostic effects. A configured socket requires Linux and a private existing
// owner directory; startup refusal precedes work. Cleanup errors remain terminal.
func Run(ctx context.Context, socket string, work func(context.Context) error) (outcome error) {
	if ctx == nil || work == nil {
		return errors.New("process diagnostics caller unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if socket == "" {
		return work(ctx)
	}
	close, err := open(ctx, socket)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, close()) }()
	return work(ctx)
}
