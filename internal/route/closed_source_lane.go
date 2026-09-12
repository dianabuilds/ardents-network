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
)

type closedSourceLane struct {
	closePayloadEmissions             uint64
	physicalWriteFailed               bool
	writeEOF                          bool
	emissions                         uint64 // Monotonic physical attempts, including failed or partial frames.
	closeStatus                       byte
	opened                            bool
	owner                             *closedSourceChannels
	id                                uint32
	end, readEnd, writeEnd            time.Time
	credit, receiveCredit, consumed   uint32
	buffer                            []byte
	active, eof, remoteClosed, closed bool
	failure                           error
	reading, writing                  sync.Mutex
	closeOnce                         sync.Once
	closeErr                          error
}

// A verified peer CLOSE makes a later, unemitted CREDIT unnecessary. This
// witness never treats a raw EOF, local close, or failed parent as peer success.
func (lane *closedSourceLane) writeWitness() (uint64, bool, bool) {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	busy := owner.active != nil && owner.active.lane == lane
	clean := !lane.closed && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
	return lane.emissions, busy, clean
}

// Cleanup can overlap an independently successful CREDIT. Count every other
// physical attempt; require all output joined and no failed physical write at
// the final clean observation. This does not relax the data/CREDIT witness.
func (lane *closedSourceLane) closeWriteWitness() (uint64, bool, bool) {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	active := owner.active != nil && owner.active.lane == lane
	payload := active && owner.active.frame.Kind != closedFrameCredit
	clean := !active && !lane.closed && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
	return lane.closePayloadEmissions, payload, clean
}
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

func (lane *closedSourceLane) enqueueLocked(frame ClosedLaneFrame, cleanup time.Time) (*closedSourceWrite, error) {
	owner := lane.owner
	size := uint64(16 + len(frame.Body))
	if err := lane.writeErrorLocked(frame.Kind == closedFrameClose); err != nil {
		return nil, err
	}
	control := frame.Kind != closedFrameBytes
	if owner.queued+size > 4<<20 || control && owner.controlsSize+size > 16<<10 {
		return nil, errors.New("closed source output queue full")
	}
	request := &closedSourceWrite{lane: lane, frame: frame, end: cleanup, done: make(chan struct{})}
	request.frame.Body = append([]byte(nil), frame.Body...)
	owner.queued += size
	if control {
		owner.controlsSize += size
		owner.controls = append(owner.controls, request)
	} else {
		// Each lane's Write holds writing until its one queued frame completes.
		owner.data = append(owner.data, request)
	}
	owner.signalLocked()
	return request, nil
}

func (lane *closedSourceLane) send(frame ClosedLaneFrame, cleanup time.Time) error {
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
	for _, queue := range []*[]*closedSourceWrite{&owner.controls, &owner.data} {
		for index, candidate := range *queue {
			if candidate != request {
				continue
			}
			*queue = append((*queue)[:index], (*queue)[index+1:]...)
			size := uint64(16 + len(request.frame.Body))
			owner.queued -= size
			if request.frame.Kind != closedFrameBytes {
				owner.controlsSize -= size
			}
			request.err = err
			close(request.done)
			owner.signalLocked()
			return
		}
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
			owner.queued -= uint64(count)
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
	err := lane.send(ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, count)}, time.Time{})
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
		count := min(len(value), closedLaneMaximum, int(lane.credit))
		lane.credit -= uint32(count)
		owner.mu.Unlock()
		if err := lane.send(ClosedLaneFrame{Kind: closedFrameBytes, Lane: lane.id, Body: value[:count]}, time.Time{}); err != nil {
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
		for _, queue := range []*[]*closedSourceWrite{&owner.controls, &owner.data} {
			for index := len(*queue) - 1; index >= 0; index-- {
				request := (*queue)[index]
				if request.lane == lane {
					owner.removeQueuedWriteLocked(request, net.ErrClosed)
				}
			}
		}
		owner.queued -= uint64(len(lane.buffer))
		clear(lane.buffer)
		lane.buffer = nil
		var activeCredit *closedSourceWrite
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind != closedFrameClose {
			deadline := time.Now()
			if owner.active.frame.Kind == closedFrameCredit {
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
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind == closedFrameOpen {
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
			terminal, lane.closeErr = lane.enqueueLocked(ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{lane.closeStatus}}, cleanupEnd)
		}
		owner.mu.Unlock()
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
		owner.mu.Unlock()
		if lane.closeErr != nil && (unemitted || errors.Is(lane.closeErr, errClosedSourceStopped)) {
			// Existing traffic errors remain at their operation/parent owner;
			// actual physical retirement failures remain cleanup failures.
			<-owner.done
			lane.closeErr = owner.retire()
		}
		// Check the retained CREDIT even if its failure ended the parent before
		// CLOSE could be queued. Whole-parent intentional stop remains distinct.
		if creditErr != nil && !errors.Is(creditErr, errClosedSourceStopped) {
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
