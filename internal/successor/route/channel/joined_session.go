package channel

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
func PrepareJoined(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *Budget) (*Session, *Lane, error) {
	if ctx == nil || conn == nil || queues == nil || !time.Now().Before(end) || remaining < ardp.HeaderSize+1 {
		return nil, nil, errors.New("route joined framing bounds invalid")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if !queues.child() {
		return nil, nil, errors.New("route joined lane capacity unavailable")
	}
	s := Prepare(ctx, conn, end, remaining, check, false, queues, Handlers{})
	s.dedicated = true
	l := s.newLaneLocked(1)
	s.next, s.last = 3, 1
	return s, l, nil
}

// prepareJoinedChild keeps the shared physical-principal budget inside its
// session owner. The JOIN caller supplies its exact connection and bounds;
// preparing a lane reserves its bounded slot but starts no reader and grants
// no Network or Admission authority.
func (s *Session) PrepareJoined(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error) (*Session, *Lane, error) {
	return PrepareJoined(ctx, conn, end, remaining, check, s.queues)
}
