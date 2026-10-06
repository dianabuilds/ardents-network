package channel

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// frameCapacityRefusal witnesses a finite child refusal before physical output.
// It cannot erase an attempted write, deadline-operation or Carrier failure.
type frameCapacityRefusal struct{ cause error }

func (e *frameCapacityRefusal) Error() string { return e.cause.Error() }
func (e *frameCapacityRefusal) Unwrap() error { return e.cause }

func localCapacityRefusal(err error) bool {
	_, ok := err.(*frameCapacityRefusal)
	return ok
}

// ConstrainQueues adds one shared child-group bound before any input or output
// can be retained. It keeps terminal CLOSE capacity until physical finish, so
// a full group cannot prevent retirement or disturb ordinary sibling lanes.
// The principal's original queue accounting remains in force as well.
func (l *Lane) ConstrainQueues(q *Budget) error {
	if q == nil || q == l.s.queues {
		return errors.New("route child queue bound invalid")
	}
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.closed || l.finished || !l.handshake || !time.Now().Before(l.end) || l.queueBound != nil || len(l.buffer) != 0 || l.queuedOutput != 0 || l.openEmitted || l.handshakeBytes != 0 || l.handshakeOutput != 0 || l.physicalAttempts != 0 {
		return errors.New("route child queue binding unavailable")
	}
	if !q.reserve(ardp.HeaderSize + 1) {
		return errors.New("route child termination capacity unavailable")
	}
	l.queueBound = q
	l.queueTermination = true
	return nil
}

// ConstrainTraffic binds complete input/output frames to a finite child budget
// before pending TLS work. OPEN is already counted; both terminal frames are
// reserved before data. Output admission runs synchronously under the framing
// lock: it must perform no I/O or call back into framing. The operation supplies
// an actual shared output owner and a prepaid once-only CLOSE permit.
func (l *Lane) ConstrainTraffic(maximum uint64, output func(uint64, bool) error) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if output == nil || l.closed || l.finished || !l.handshake || !time.Now().Before(l.end) || l.trafficLimit != 0 || l.chargeOutput != nil || len(l.buffer) != 0 || l.queuedOutput != 0 || l.openEmitted || l.handshakeBytes != 0 || l.handshakeOutput != 0 || l.physicalAttempts != 0 || maximum < l.trafficUsed || maximum-l.trafficUsed < 2*(ardp.HeaderSize+1) {
		return errors.New("route child traffic binding unavailable")
	}
	l.trafficLimit = maximum
	l.trafficUsed += 2 * (ardp.HeaderSize + 1)
	l.chargeOutput = output
	return nil
}
