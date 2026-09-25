//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

type closedSourceLane struct {
	reservedControl                   bool
	closePayloadEmissions             uint64
	physicalWriteFailed               bool
	writeEOF                          bool
	emissions                         uint64 // Monotonic physical attempts, including failed or partial frames.
	closeStatus                       byte
	terminalWriters                   uint32
	opened                            bool
	owner                             *closedSourceChannels
	id                                uint32
	end, readEnd, writeEnd            time.Time
	credit, receiveCredit, consumed   uint32
	buffer                            []byte
	active, eof, remoteClosed, closed bool
	terminalRead                      bool
	receivedData                      bool
	failure                           error
	reading, writing                  sync.Mutex
	closeOnce                         sync.Once
	closeErr                          error
}

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

func waitClosedSourceChange(changed <-chan struct{}, deadline time.Time) error {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-changed:
		return nil
	case <-timer.C:
		return os.ErrDeadlineExceeded
	}
}

func (lane *closedSourceLane) Read(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	lane.reading.Lock()
	defer lane.reading.Unlock()
	owner := lane.owner
	for {
		owner.mu.Lock()
		drain := owner.retainClosedRead && owner.terminal == io.EOF && (lane.failure == nil || lane.failure == io.EOF) && (len(lane.buffer) != 0 || lane.remoteClosed)
		if lane.closed || owner.terminal != nil && !drain {
			cause := owner.terminal
			if owner.retainClosedRead && !lane.closed {
				if lane.remoteClosed {
					cause = errors.Join(cause, errors.New("closed JOIN failed after peer CLOSE"))
				} else {
					cause = errors.Join(cause, errors.New("closed JOIN failed before peer CLOSE"))
				}
			}
			owner.mu.Unlock()
			return 0, errors.Join(net.ErrClosed, cause)
		}
		if !time.Now().Before(lane.readEnd) {
			owner.mu.Unlock()
			return 0, os.ErrDeadlineExceeded
		}
		if len(lane.buffer) > 0 {
			count := copy(value, lane.buffer)
			clear(lane.buffer[:count])
			lane.buffer = lane.buffer[count:]
			owner.releaseQueuedLocked(uint64(count))
			lane.consumed += uint32(count)
			if owner.retainClosedRead {
				owner.signalLocked()
			}
			owner.mu.Unlock()
			// Already consumed bytes remain returned even if the later CREDIT
			// write fails. That failure is terminal for the next operation.
			if err := lane.returnCredit(); err != nil {
				owner.mu.Lock()
				lane.failure = err
				owner.signalLocked()
				owner.mu.Unlock()
			}
			return count, nil
		}
		err := lane.failure
		if err == nil && lane.eof {
			err = io.EOF
		}
		if err != nil {
			if err == io.EOF {
				lane.terminalRead = true
				owner.signalLocked()
			}
			owner.mu.Unlock()
			return 0, err
		}
		changed, deadline := owner.changed, lane.readEnd
		owner.mu.Unlock()
		if err := waitClosedSourceChange(changed, deadline); err != nil {
			return 0, err
		}
	}
}

func (lane *closedSourceLane) activate() error {
	lane.owner.mu.Lock()
	lane.active = true
	lane.owner.mu.Unlock()
	return lane.returnCredit()
}

func (lane *closedSourceLane) returnCredit() error {
	owner := lane.owner
	owner.mu.Lock()
	if !lane.active || lane.remoteClosed || lane.closed || lane.consumed == 0 || lane.failure != nil {
		owner.mu.Unlock()
		return nil
	}
	count := lane.consumed
	lane.consumed = 0
	lane.receiveCredit += count
	owner.mu.Unlock()
	err := lane.send(ardp.Frame{Kind: ardp.KindCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, count)}, time.Time{})
	// A joined outer parent can fail after these bytes were accepted but before
	// CREDIT enters its queue. No credit is usable after that terminal state;
	// retain the parent cause for the reader instead of replacing it with a
	// local queue-capacity error.
	if err != nil && owner.retainClosedRead && owner.queueParent != nil {
		owner.queueParent.mu.Lock()
		parentEnded := owner.queueParent.terminal != nil
		owner.queueParent.mu.Unlock()
		if parentEnded {
			return nil
		}
	}
	// CLOSE can win after the bytes were consumed and before credit queues.
	owner.mu.Lock()
	retired := lane.remoteClosed || lane.closed
	owner.mu.Unlock()
	if retired {
		return nil
	}
	return err
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

func (lane *closedSourceLane) Close() error {
	lane.closeOnce.Do(func() {
		cleanupEnd := time.Now().Add(time.Second)
		owner := lane.owner
		owner.mu.Lock()
		lane.closed = true
		// Release this lane's queued readers/writers even while a sibling owns
		// the physical writer. An already active frame must finish or retire
		// the parent because its wire prefix may have been emitted.
		for _, queue := range []*[]*closedSourceWrite{&owner.terminals, &owner.controls, &owner.data} {
			for index := len(*queue) - 1; index >= 0; index-- {
				request := (*queue)[index]
				if request.lane == lane {
					owner.removeQueuedWriteLocked(request, net.ErrClosed)
				}
			}
		}
		owner.releaseQueuedLocked(uint64(len(lane.buffer)))
		clear(lane.buffer)
		lane.buffer = nil
		var activeCredit *closedSourceWrite
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind != ardp.KindClose {
			deadline := time.Now()
			if owner.active.frame.Kind == ardp.KindCredit {
				activeCredit = owner.active
				// This already emitted control frame can finish before CLOSE.
				// Cutting it short needlessly retires the shared physical prefix.
				// Never extend its original write authority or the cleanup bound.
				deadline = cleanupEnd
				if lane.writeEnd.Before(deadline) {
					deadline = lane.writeEnd
				}
				owner.active.end = deadline
			}
			_ = owner.parent.SetWriteDeadline(deadline)
		}
		var opening <-chan struct{}
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind == ardp.KindOpen {
			opening = owner.active.done
		}
		owner.signalLocked()
		owner.mu.Unlock()
		if opening != nil {
			<-opening
		}
		var creditErr error
		if activeCredit != nil {
			<-activeCredit.done
			creditErr = activeCredit.err
		}
		owner.mu.Lock()
		opened, parentEnded := lane.opened, owner.terminal != nil
		peerEndedJoin := owner.retainClosedRead && lane.remoteClosed
		var terminal *closedSourceWrite
		// Observe parent retirement and queue cleanup atomically. If that parent
		// ends before emission starts, its joined physical retirement replaces
		// an undelivered child CLOSE without inventing a cleanup failure.
		if opened && !parentEnded && !peerEndedJoin {
			terminal, lane.closeErr = lane.enqueueLocked(ardp.Frame{Kind: ardp.KindClose, Lane: lane.id, Body: []byte{lane.closeStatus}}, cleanupEnd)
		}
		owner.mu.Unlock()
		if terminal == nil && errors.Is(lane.closeErr, errClosedSourceOutputQueueFull) && owner.retainClosedRead && owner.queueParentEnded() {
			// The outer joined owner already published retirement. Its cause is
			// returned by the joined stream; an inner CLOSE can no longer enter
			// that parent's queue and must not invent a local cleanup failure.
			lane.closeErr = nil
		}
		if terminal != nil {
			lane.closeErr = owner.awaitWrite(terminal)
			if lane.closeErr != nil {
				lane.closeErr = errors.Join(errors.New("closed source CLOSE write failed"), lane.closeErr)
			}
		}
		if terminal != nil && terminal.unwritten {
			// The lower peer has closed cleanly and no physical frame began for
			// this CLOSE. Join that parent before releasing this unused request.
			owner.fail(terminal.err)
		}
		owner.mu.Lock()
		unemitted := owner.terminal != nil && (terminal == nil || !terminal.attempted || terminal.unwritten)
		joinedPeerEnd := owner.retainClosedRead && owner.terminal == io.EOF &&
			lane.remoteClosed && lane.failure == io.EOF && owner.framedParent != nil
		owner.mu.Unlock()
		joinedLowerClose, joinedLowerCredit := false, false
		if (lane.closeErr != nil || creditErr != nil) && joinedPeerEnd {
			<-owner.done
			if lane.closeErr != nil {
				_, active, clean := owner.framedParent.closeWriteWitness()
				joinedLowerClose = !active && clean
			}
			if creditErr != nil && errors.Is(creditErr, io.EOF) {
				_, active, clean := owner.framedParent.writeWitness()
				joinedLowerCredit = !active && clean
			}
		}
		if lane.closeErr != nil && (unemitted || joinedLowerClose || errors.Is(lane.closeErr, ErrClosedSourceStopped)) {
			// Existing traffic errors remain at their operation/parent owner;
			// actual physical retirement failures remain cleanup failures.
			if !joinedPeerEnd {
				<-owner.done
			}
			lane.closeErr = owner.retire()
		}
		// Check the retained CREDIT even if its failure ended the parent before
		// CLOSE could be queued. Whole-parent intentional stop remains distinct.
		if creditErr != nil && !joinedLowerCredit && !errors.Is(creditErr, ErrClosedSourceStopped) {
			lane.closeErr = errors.Join(lane.closeErr, errors.Join(errors.New("closed source in-flight CREDIT write failed"), creditErr))
		}
		if lane.closeErr != nil {
			lane.closeErr = errors.Join(ErrClosedSourceCleanup, errors.New("closed source child terminal write failed"), lane.closeErr)
			owner.fail(lane.closeErr)
			<-owner.done
		}
		// Join existing I/O and keep later calls out until the lane has been
		// removed and the retained parent's idle state has been committed.
		lane.writing.Lock()
		defer lane.writing.Unlock()
		lane.reading.Lock()
		defer lane.reading.Unlock()
		owner.mu.Lock()
		// Capture peer refusal atomically with removal, including one received
		// while our terminal write or existing application I/O was finishing.
		var peerErr error
		if owner.retainClosedRead && lane.remoteClosed && lane.failure != io.EOF {
			peerErr = lane.failure
		}
		delete(owner.lanes, lane.id)
		if lane.active {
			owner.idleUntil = time.Now().Add(closedSourceRetention)
		}
		owner.signalLocked()
		owner.mu.Unlock()
		if peerErr != nil {
			lane.closeErr = errors.Join(lane.closeErr, ErrClosedSourceCleanup, peerErr)
			owner.fail(lane.closeErr)
			<-owner.done
		}
	})
	return lane.closeErr
}

func (lane *closedSourceLane) bound(end time.Time) time.Time {
	if end.IsZero() || end.After(lane.end) {
		return lane.end
	}
	return end
}
func (lane *closedSourceLane) SetDeadline(end time.Time) error {
	if err := lane.SetReadDeadline(end); err != nil {
		return err
	}
	return lane.SetWriteDeadline(end)
}
func (lane *closedSourceLane) SetReadDeadline(end time.Time) error {
	lane.owner.mu.Lock()
	lane.readEnd = lane.bound(end)
	lane.owner.signalLocked()
	lane.owner.mu.Unlock()
	return nil
}
func (lane *closedSourceLane) SetWriteDeadline(end time.Time) error {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	lane.writeEnd = lane.bound(end)
	owner.signalLocked()
	if owner.active != nil && owner.active.lane == lane && owner.active.end.IsZero() {
		return owner.parent.SetWriteDeadline(lane.writeEnd)
	}
	return nil
}
func (lane *closedSourceLane) LocalAddr() net.Addr  { return lane.owner.parent.LocalAddr() }
func (lane *closedSourceLane) RemoteAddr() net.Addr { return lane.owner.parent.RemoteAddr() }
