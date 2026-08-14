package route

import (
	"context"
	"errors"
	"fmt"
	"net"
)

func bindListenerLifetime(ctx context.Context, listener *net.TCPListener, role string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New(role + " Route listener lifetime is unbounded")
	}
	if err := listener.SetDeadline(deadline); err != nil {
		return fmt.Errorf("bound %s Route listener lifetime: %w", role, err)
	}
	return nil
}
