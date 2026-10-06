package channel

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// FinishRole completes an outgoing forwarding role after its work owner has
// joined all terminal borrowers. It half-closes the inner direction, joins its
// sole reader and waits for the exact lower peer CLOSE(0) before physical
// retirement. A raw EOF alone is insufficient. Close still joins and returns
// the retained physical result; cancellation always interrupts through Retire.
// This does not finish a JOIN, grant payload success or renew an original bound.
func (s *Session) FinishRole() (result error) {
	lower := lowerFramingLane(s.conn)
	writer, ok := s.conn.(interface{ CloseWrite() error })
	if lower == nil || !ok {
		return errors.New("route nested role termination unavailable")
	}
	if s.check != nil {
		if err := s.check(); err != nil {
			s.Retire(err)
			return err
		}
	}
	s.mu.Lock()
	if s.stopped || s.finishingRole || s.dedicated || s.open != nil || s.live != 0 || s.exchange != nil || s.ctx.Err() != nil {
		s.mu.Unlock()
		return errors.New("route role termination unavailable")
	}
	s.finishingRole = true
	end := minDeadline(s.end, time.Now().Add(time.Second))
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() { defer close(interrupted); s.Retire(s.ctx.Err()) })
	defer func() {
		if !stop() {
			<-interrupted
		}
		if result != nil {
			s.Retire(result)
		}
	}()
	// crypto/tls.CloseWrite applies its own five-second write deadline. The
	// original lower lane must enforce the earlier cleanup horizon itself.
	if err := lower.Bound(end); err != nil {
		return err
	}
	if err := s.conn.SetReadDeadline(end); err != nil {
		return err
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := writer.CloseWrite(); err != nil {
		return err
	}
	// TLS close-notify terminates the inner byte grammar, but the relay's
	// opposite copier also needs the original lower direction to end. Its
	// canonical EOF follows all preceding TLS bytes on that same lane.
	// TLS has set its underlying data-write deadline to now; restore only the
	// already shortened cleanup horizon for this directional terminal frame.
	if err := lower.SetWriteDeadline(end); err != nil {
		return err
	}
	if err := lower.CloseWrite(); err != nil {
		return err
	}
	<-s.readerDone
	s.mu.Lock()
	input, stopped := s.failure, s.stopped
	s.mu.Unlock()
	if input != io.EOF || stopped {
		return errors.Join(errors.New("route role reverse termination required"), input, s.ctx.Err())
	}
	if err := lower.waitPeerRetirement(s.ctx); err != nil {
		return err
	}
	if s.check != nil {
		if err := s.check(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.failure != io.EOF || s.ctx.Err() != nil {
		return errors.Join(errors.New("route original role retired during termination"), s.failure, s.ctx.Err())
	}
	// Discharge only this reverse EOF after actual lower CLOSE(0), joined lower
	// writes and original-authority checks. Close retains all independent
	// physical write, close and release failures.
	s.failure = nil
	return nil
}

func (l *Lane) waitPeerRetirement(ctx context.Context) error {
	for {
		l.s.mu.Lock()
		peer := l.peerClosed
		changed, end := l.changed, l.end
		closed := l.localClosed || l.s.stopped || l.peerRefused
		cause := errors.Join(l.cause, l.s.failure, ctx.Err())
		l.s.mu.Unlock()
		if closed || cause != nil {
			return errors.Join(net.ErrClosed, cause)
		}
		if peer {
			// Peer CLOSE seals new lane output under the same lock used to add
			// writes. Join already selected CREDIT before checking its witness.
			l.writes.Wait()
			witness := l.retirementWitness()
			if witness.clean && !witness.local && !witness.busy && ctx.Err() == nil {
				return nil
			}
			l.s.mu.Lock()
			cause := errors.Join(l.cause, l.s.failure)
			l.s.mu.Unlock()
			return errors.Join(errors.New("route lower peer retirement failed"), cause, ctx.Err())
		}
		if err := waitLane(changed, end); err != nil {
			return err
		}
	}
}
