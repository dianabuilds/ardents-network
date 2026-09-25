//go:build linux

package route

import (
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

var errClosedSourceOutputQueueFull = errors.New("closed source output queue full")

func (lane *closedSourceLane) writeErrorLocked(terminal bool) error {
	if lane.owner.terminal != nil {
		return lane.owner.terminal
	}
	if terminal {
		return nil
	}
	if lane.closed {
		return net.ErrClosed
	}
	if lane.failure != nil {
		return lane.failure
	}
	return nil
}

func (lane *closedSourceLane) enqueueLocked(frame ardp.Frame, cleanup time.Time) (*closedSourceWrite, error) {
	owner := lane.owner
	size := uint64(16 + len(frame.Body))
	if err := lane.writeErrorLocked(frame.Kind == ardp.KindClose); err != nil {
		return nil, err
	}
	control := frame.Kind != ardp.KindBytes
	terminal := frame.Kind == ardp.KindClose || lane.terminalWriters != 0
	if control && owner.controlsSize+size > 16<<10 || !owner.reserveQueuedLocked(size, control) {
		return nil, errClosedSourceOutputQueueFull
	}
	request := &closedSourceWrite{lane: lane, frame: frame, end: cleanup, done: make(chan struct{}), control: control, terminal: terminal}
	request.frame.Body = append([]byte(nil), frame.Body...)
	if terminal {
		if control {
			owner.controlsSize += size
		}
		owner.terminals = append(owner.terminals, request)
	} else if control {
		owner.controlsSize += size
		owner.controls = append(owner.controls, request)
	} else {
		// Each lane's Write holds writing until its one queued frame completes.
		owner.data = append(owner.data, request)
	}
	owner.signalLocked()
	return request, nil
}

func (lane *closedSourceLane) send(frame ardp.Frame, cleanup time.Time) error {
	lane.owner.mu.Lock()
	request, err := lane.enqueueLocked(frame, cleanup)
	lane.owner.mu.Unlock()
	if err != nil {
		return err
	}
	return lane.owner.awaitWrite(request)
}

func (owner *closedSourceChannels) awaitWrite(request *closedSourceWrite) error {
	for {
		owner.mu.Lock()
		select {
		case <-request.done:
			owner.mu.Unlock()
			return request.err
		default:
		}
		end := request.lane.writeEnd
		if !request.end.IsZero() {
			end = request.end
		}
		if !time.Now().Before(end) {
			if owner.active == request {
				_ = owner.parent.SetWriteDeadline(time.Now())
			} else {
				owner.removeQueuedWriteLocked(request, os.ErrDeadlineExceeded)
			}
			owner.mu.Unlock()
			<-request.done
			return request.err
		}
		changed := owner.changed
		owner.mu.Unlock()
		timer := time.NewTimer(time.Until(end))
		select {
		case <-request.done:
		case <-changed:
		case <-timer.C:
		}
		timer.Stop()
	}
}

func (owner *closedSourceChannels) removeQueuedWriteLocked(request *closedSourceWrite, err error) {
	for _, queue := range []*[]*closedSourceWrite{&owner.terminals, &owner.controls, &owner.data} {
		for index, candidate := range *queue {
			if candidate != request {
				continue
			}
			*queue = append((*queue)[:index], (*queue)[index+1:]...)
			size := uint64(16 + len(request.frame.Body))
			owner.releaseQueuedLocked(size)
			if request.control {
				owner.controlsSize -= size
			}
			request.err = err
			close(request.done)
			owner.signalLocked()
			return
		}
	}
}

// beginTerminalWrite carries an authenticated inner terminal's scheduling
// class through its encrypted TLS record. It changes neither wire bytes nor
// the request's ordinary queue reservation.
func (lane *closedSourceLane) beginTerminalWrite() func() {
	owner := lane.owner
	owner.mu.Lock()
	lane.terminalWriters++
	owner.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			owner.mu.Lock()
			lane.terminalWriters--
			owner.mu.Unlock()
		})
	}
}

func (lane *closedSourceLane) Write(value []byte) (int, error) {
	lane.writing.Lock()
	defer lane.writing.Unlock()
	owner := lane.owner
	written := 0
	for len(value) != 0 {
		owner.mu.Lock()
		if lane.writeEOF {
			owner.mu.Unlock()
			return written, net.ErrClosed
		}
		if err := lane.writeErrorLocked(false); err != nil {
			owner.mu.Unlock()
			return written, err
		}
		if !time.Now().Before(lane.writeEnd) {
			owner.mu.Unlock()
			return written, os.ErrDeadlineExceeded
		}
		if lane.credit == 0 {
			changed, deadline := owner.changed, lane.writeEnd
			owner.mu.Unlock()
			if err := waitClosedSourceChange(changed, deadline); err != nil {
				return written, err
			}
			continue
		}
		count := min(len(value), ardp.MaximumBodySize, int(lane.credit))
		lane.credit -= uint32(count)
		owner.mu.Unlock()
		if err := lane.send(ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: value[:count]}, time.Time{}); err != nil {
			return written, err
		}
		written += count
		value = value[count:]
	}
	return written, nil
}
