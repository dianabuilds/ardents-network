package node

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// A deadline update can interrupt only its own in-flight payload frame. The
// next writer reads its live deadline after acquiring serialization, so a
// queued frame cannot restore a deadline invalidated by local cancellation.
// Partial-frame failure poisons the physical framing boundary and closes it.
type closedOuterWriter struct {
	connection net.Conn
	writer     sync.Mutex
	state      sync.Mutex
	active     bool
	lane       uint32
}

func (owner *closedOuterWriter) write(frame route.ClosedLaneFrame, deadline func() time.Time) error {
	owner.writer.Lock()
	defer owner.writer.Unlock()
	owner.state.Lock()
	end := deadline()
	// Cancellation while waiting for serialization has emitted no frame.
	// Refuse that owner without damaging other lanes on the same Carrier.
	// After a physical write is attempted, every failure still poisons it.
	if !end.IsZero() && !time.Now().Before(end) {
		owner.state.Unlock()
		return os.ErrDeadlineExceeded
	}
	owner.active, owner.lane = frame.Kind != 9, frame.Lane
	err := owner.connection.SetWriteDeadline(end)
	owner.state.Unlock()
	if err == nil {
		err = route.WriteClosedLaneFrame(owner.connection, frame)
	}
	owner.state.Lock()
	owner.active = false
	owner.state.Unlock()
	if err != nil {
		_ = owner.connection.Close()
	}
	return err
}

func (owner *closedOuterWriter) update(lane uint32, deadline time.Time) error {
	owner.state.Lock()
	defer owner.state.Unlock()
	if owner.active && owner.lane == lane {
		return owner.connection.SetWriteDeadline(deadline)
	}
	return nil
}
