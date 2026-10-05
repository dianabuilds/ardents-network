//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// prepareJoinedSession reserves and installs the sole lane before an accepted
// RESULT can be emitted. Its owner starts reading only after the actual paired
// result barrier, or closes this preparation on any refusal.
func prepareJoinedSession(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *queueBudget) (*session, *lane, error) {
	if ctx == nil || conn == nil || queues == nil || !time.Now().Before(end) || remaining < ardp.HeaderSize+1 {
		return nil, nil, errors.New("route joined framing bounds invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if !queues.child() {
		return nil, nil, errors.New("route joined lane capacity unavailable")
	}
	s := prepareSession(ctx, conn, end, remaining, check, false, queues, nil)
	s.dedicated = true
	l := s.newLaneLocked(1)
	s.next, s.last = 3, 1
	return s, l, nil
}
