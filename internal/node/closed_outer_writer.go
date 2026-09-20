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
	active     *closedOuterWriteRequest
	terminals  []*closedOuterWriteRequest
	controls   []*closedOuterWriteRequest
	data       []*closedOuterWriteRequest
	dataDue    bool
	running    bool
}

type closedOuterWriteRequest struct {
	frame             route.ClosedLaneFrame
	deadline          func() time.Time
	control, terminal bool
	done              chan struct{}
	err               error
}

func (owner *closedOuterWriter) write(frame route.ClosedLaneFrame, deadline func() time.Time, control, terminal bool) error {
	request := &closedOuterWriteRequest{frame: frame, deadline: deadline, control: control, terminal: terminal, done: make(chan struct{})}
	owner.state.Lock()
	switch {
	case terminal:
		owner.terminals = append(owner.terminals, request)
	case control:
		owner.controls = append(owner.controls, request)
	default:
		owner.data = append(owner.data, request)
	}
	// A local terminal may interrupt only an ordinary payload already active on
	// that same lane. CREDIT and nested terminal records must finish so their
	// shared framing boundary remains valid; sibling lanes remain independent.
	if terminal && owner.active != nil && owner.active.frame.Lane == frame.Lane && !owner.active.control && !owner.active.terminal {
		_ = owner.connection.SetWriteDeadline(time.Now())
	}
	if !owner.running {
		owner.running = true
		go owner.drain()
	}
	owner.state.Unlock()
	<-request.done
	return request.err
}

func (owner *closedOuterWriter) nextLocked() *closedOuterWriteRequest {
	if len(owner.terminals) != 0 {
		request := owner.terminals[0]
		owner.terminals = owner.terminals[1:]
		return request
	}
	if len(owner.controls) != 0 && (len(owner.data) == 0 || !owner.dataDue) {
		request := owner.controls[0]
		owner.controls = owner.controls[1:]
		owner.dataDue = true
		return request
	}
	if len(owner.data) != 0 {
		request := owner.data[0]
		owner.data = owner.data[1:]
		owner.dataDue = false
		return request
	}
	return nil
}

func (owner *closedOuterWriter) drain() {
	for {
		owner.writer.Lock()
		owner.state.Lock()
		request := owner.nextLocked()
		if request == nil {
			owner.running = false
			owner.state.Unlock()
			owner.writer.Unlock()
			return
		}
		owner.active = request
		end := request.deadline()
		// Cancellation while waiting for serialization has emitted no frame.
		// Refuse that owner without damaging other lanes on the same Carrier.
		// After a physical write is attempted, every failure still poisons it.
		if !end.IsZero() && !time.Now().Before(end) {
			request.err = os.ErrDeadlineExceeded
			owner.active = nil
			close(request.done)
			owner.state.Unlock()
			owner.writer.Unlock()
			continue
		}
		err := owner.connection.SetWriteDeadline(end)
		owner.state.Unlock()
		if err == nil {
			err = route.WriteClosedLaneFrame(owner.connection, request.frame)
		}
		owner.state.Lock()
		owner.active = nil
		request.err = err
		close(request.done)
		owner.state.Unlock()
		owner.writer.Unlock()
		if err != nil {
			_ = owner.connection.Close()
		}
	}
}

func (owner *closedOuterWriter) update(lane uint32, deadline time.Time) error {
	owner.state.Lock()
	defer owner.state.Unlock()
	if owner.active != nil && owner.active.frame.Kind != 9 && owner.active.frame.Lane == lane {
		return owner.connection.SetWriteDeadline(deadline)
	}
	return nil
}
