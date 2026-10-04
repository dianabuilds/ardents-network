//go:build linux

package transport

import (
	"crypto/tls"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

type framingWitness struct {
	attempts, payload               uint64
	busy, payloadBusy, clean, local bool
}

func lowerFramingLane(conn net.Conn) *lane {
	for {
		switch current := conn.(type) {
		case *retiredConn:
			conn = current.Conn
		case *tls.Conn:
			conn = current.NetConn()
		case *lane:
			return current
		default:
			return nil
		}
	}
}

func (l *lane) retirementWitness() framingWitness {
	if l == nil {
		return framingWitness{}
	}
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	busy := l.s.active == l
	return framingWitness{attempts: l.physicalAttempts, payload: l.payloadAttempts, busy: busy, payloadBusy: busy && l.s.activeKind != ardp.KindCredit, local: l.localClosed, clean: l.peerClosed && !l.peerRefused && l.s.failure == nil && !l.s.stopped && l.s.ctx.Err() == nil && !l.physicalWriteFailed}
}

// A specific lower lane's authenticated CLOSE(0), unchanged physical attempts
// across the complete encoded/TLS write and no failed/active writer are the
// only evidence for an unnecessary unemitted control. Raw EOF, local closure,
// refused CLOSE, a partial write or an earlier failed CREDIT cannot qualify.
func cleanUnemittedRetirement(kind uint8, before, after framingWitness) bool {
	if before.local || !after.clean || after.busy {
		return false
	}
	switch kind {
	case ardp.KindCredit:
		return before.attempts == after.attempts
	case ardp.KindClose:
		return !after.local && !before.payloadBusy && before.payload == after.payload
	default:
		return false
	}
}
