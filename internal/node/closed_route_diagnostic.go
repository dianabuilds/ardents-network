package node

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// emitClosedRouteDiagnostic records one bounded local failure category. It
// deliberately omits lane, peer, address, State, TLS, and payload values.
// Diagnostics cannot alter the Route result when their sink is unavailable.
func emitClosedRouteDiagnostic(config runtimeConfig, reason string) {
	if config.Emit == nil || reason == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = config.Emit(ctx, Event{Schema: eventSchema, Kind: "route-diagnostic", State: "FAILED", At: config.now().UTC(), Reason: reason})
}

func closedRouteDiagnosticCause(err error) string {
	if err == nil {
		return "none"
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, io.EOF):
		return "eof"
	case errors.Is(err, net.ErrClosed):
		return "closed"
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return "timeout"
	}
	return "other"
}
