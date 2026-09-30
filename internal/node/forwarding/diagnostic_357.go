package forwarding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var diagnostic357ForwardIDs atomic.Uint64

func diagnostic357Forward(owner string, id uint64, phase string, kind uint8, err error) {
	var timed net.Error
	category := "other"
	if err == nil {
		category = "none"
	}
	// Only fixed matching categories are emitted; never emit error text.
	for _, candidate := range []struct{ match, label string }{
		{"closed forwarding child is unavailable", "child-unavailable"},
		{"closed forwarding child is reused", "child-reused"},
		{"closed forwarding child retired", "child-retired"},
		{"closed bootstrap child cannot accept private admission", "bootstrap-admit"},
		{"credit", "credit-rule"}, {"expired", "expired"}, {"deadline", "deadline"},
	} {
		if err != nil && strings.Contains(err.Error(), candidate.match) {
			category = candidate.label
			break
		}
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-357-peer] time=%s owner=%s id=%d phase=%s kind=%d category=%s eof=%t timeout=%t canceled=%t closed=%t\n", time.Now().UTC().Format(time.RFC3339Nano), owner, id, phase, kind, category, errors.Is(err, io.EOF), errors.As(err, &timed) && timed.Timeout(), errors.Is(err, context.Canceled), errors.Is(err, net.ErrClosed))
}
