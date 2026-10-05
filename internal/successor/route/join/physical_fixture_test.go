package join

import (
	"context"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"net"
	"time"
)

// This fixture starts only real physical framing; it supplies no authority,
// admission or successful receiving result.
func newJoinedSession(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *framing.Budget) (*framing.Session, *framing.Lane, error) {
	s, l, err := framing.PrepareJoined(ctx, conn, end, remaining, check, queues)
	if err != nil {
		return nil, nil, err
	}
	s.Start()
	return s, l, nil
}
