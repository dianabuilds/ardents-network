//go:build linux

package route

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

var errClosedSourceStopped = errors.Join(net.ErrClosed, errors.New("closed source owner stopped"))

// closedSourceChannels owns the Interior channel's reader and serialized
// writer. Lanes carry opaque TLS; only the caller selecting an authenticated
// role may create one. No lane owns or changes the parent's read deadline.
type closedSourceChannels struct {
	retainClosedRead                  bool // Joined clients drain received bytes before releasing their reservation.
	chargeTerminal                    bool // Joined class-2 admission counts terminal frames too.
	parent                            net.Conn
	framing                           *closedRoleChildStream
	framedParent                      *closedSourceLane
	retire                            func() error
	end                               time.Time
	idleUntil                         time.Time
	mu                                sync.Mutex
	changed                           chan struct{}
	lanes                             map[uint32]*closedSourceLane
	last                              uint32
	queued, controlsSize, transferred uint64
	controls, data                    []*closedSourceWrite
	active                            *closedSourceWrite
	terminal                          error
	workers                           sync.WaitGroup
	done                              chan struct{}
	closeOnce                         sync.Once
	closeErr                          error
}

type closedSourceWrite struct {
	unwritten bool // A lower framing witness proves no physical frame began.
	attempted bool // Guarded by owner.mu; physical frame emission began.
	lane      *closedSourceLane
	frame     ClosedLaneFrame
	end       time.Time // Nonzero only for terminal cleanup.
	done      chan struct{}
	err       error
}

func newClosedSourceChannelOwner(parent net.Conn, end time.Time, retire func() error) *closedSourceChannels {
	return &closedSourceChannels{parent: parent, retire: retire, end: end, changed: make(chan struct{}), lanes: make(map[uint32]*closedSourceLane), done: make(chan struct{}), idleUntil: time.Now().Add(closedSourceRetention)}
}

func (owner *closedSourceChannels) start() {
	owner.workers.Add(3)
	go owner.read()
	go owner.write()
	go owner.expire()
	go func() { owner.workers.Wait(); close(owner.done) }()
}

func (owner *closedSourceChannels) signalLocked() {
	close(owner.changed)
	owner.changed = make(chan struct{})
}

func (owner *closedSourceChannels) fail(err error) {
	owner.mu.Lock()
	if owner.terminal == nil {
		owner.terminal = err
		for _, queue := range [][]*closedSourceWrite{owner.controls, owner.data} {
			for _, request := range queue {
				request.err = err
				close(request.done)
				owner.queued -= uint64(16 + len(request.frame.Body))
			}
		}
		owner.controls, owner.data = nil, nil
		if owner.active != nil && owner.active.frame.Kind != closedFrameBytes {
			owner.controlsSize = uint64(16 + len(owner.active.frame.Body))
		} else {
			owner.controlsSize = 0
		}
		owner.signalLocked()
	}
	owner.mu.Unlock()
	_ = owner.retire()
}

// stop records intentional whole-parent retirement before interrupting physical I/O.
func (owner *closedSourceChannels) stop() { owner.fail(errClosedSourceStopped) }

func (owner *closedSourceChannels) Close() error {
	owner.closeOnce.Do(func() {
		owner.stop()
		<-owner.done
		owner.closeErr = owner.retire()
		owner.mu.Lock()
		for _, lane := range owner.lanes {
			owner.queued -= uint64(len(lane.buffer))
			clear(lane.buffer)
			lane.buffer = nil
		}
		owner.lanes = nil
		owner.mu.Unlock()
	})
	return owner.closeErr
}

func (owner *closedSourceChannels) open(ctx context.Context, open ClosedOpen, pending time.Time) (*closedSourceLane, error) {
	body, err := EncodeClosedOpen(open)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	if owner.terminal != nil {
		cause := owner.terminal
		owner.mu.Unlock()
		return nil, errors.Join(errors.New("closed source parent failed before child open"), cause)
	}
	if !time.Now().Before(pending) || pending.After(open.Deadline) ||
		open.Deadline.After(owner.end) || len(owner.lanes) == 0 && !time.Now().Before(owner.idleUntil) || len(owner.lanes) >= 256 || owner.last > ^uint32(0)-2 {
		owner.mu.Unlock()
		return nil, errors.New("closed source child unavailable")
	}
	id := owner.last + 2
	if owner.last == 0 {
		id = 1
	}
	owner.last = id
	lane := &closedSourceLane{owner: owner, id: id, end: open.Deadline,
		readEnd: pending, writeEnd: pending, closeStatus: 5, credit: 64 << 10, receiveCredit: 64 << 10}
	owner.lanes[id] = lane
	// Assign the monotonic ID and queue its OPEN under the same lock. A
	// concurrent caller cannot put a higher ID on the wire first.
	request, err := lane.enqueueLocked(ClosedLaneFrame{Kind: closedFrameOpen, Lane: id, Body: body}, time.Time{})
	if err != nil {
		delete(owner.lanes, id)
		owner.mu.Unlock()
		return nil, err
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		_ = lane.SetWriteDeadline(time.Now())
	})
	err = owner.awaitWrite(request)
	if !stop() {
		<-interrupted
	}
	if err != nil || ctx.Err() != nil {
		return nil, errors.Join(err, ctx.Err(), lane.Close())
	}
	return lane, nil
}

func (owner *closedSourceChannels) read() {
	defer owner.workers.Done()
	for {
		frame, err := ReadClosedLaneFrame(owner.parent)
		if err != nil {
			owner.fail(err)
			return
		}
		if err := owner.receive(frame); err != nil {
			owner.fail(err)
			return
		}
	}
}

func (owner *closedSourceChannels) receive(frame ClosedLaneFrame) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.terminal != nil {
		return owner.terminal
	}
	if frame.Lane == 0 || frame.Lane%2 == 0 || frame.Lane > owner.last {
		return errors.New("closed source peer used unallocated lane")
	}
	owner.transferred += uint64(16 + len(frame.Body))
	if owner.transferred > 32<<20 {
		return errors.New("closed source parent byte reserve exhausted")
	}
	lane := owner.lanes[frame.Lane]
	if lane == nil || (lane.closed && !(owner.retainClosedRead && frame.Kind == closedFrameClose)) {
		// A locally closed lane can still have frames in flight. Its ID is
		// never reused; discarding them grants no new credit or work.
		return nil
	}
	switch frame.Kind {
	case closedFrameBytes:
		size := uint64(len(frame.Body))
		if lane.eof || lane.remoteClosed || size > uint64(lane.receiveCredit) || owner.queued+size > 4<<20 {
			return errors.New("closed source receive allowance exceeded")
		}
		lane.receiveCredit -= uint32(size)
		lane.buffer = append(lane.buffer, frame.Body...)
		owner.queued += size
	case closedFrameCredit:
		increment := binary.BigEndian.Uint32(frame.Body)
		if increment == 0 || increment > 64<<10-lane.credit {
			return errors.New("closed source peer credit exceeded")
		}
		lane.credit += increment
	case closedFrameEOF:
		if lane.eof || lane.remoteClosed {
			return errors.New("closed source duplicate EOF")
		}
		lane.eof = true
	case closedFrameClose:
		if lane.remoteClosed {
			return errors.New("closed source duplicate CLOSE")
		}
		lane.remoteClosed = true
		lane.failure = io.EOF
		if frame.Body[0] != 0 {
			lane.failure = errors.New("closed source child refused")
		}
	default:
		return errors.New("closed source child frame unavailable")
	}
	owner.signalLocked()
	return nil
}

func (owner *closedSourceChannels) write() {
	defer owner.workers.Done()
	for {
		owner.mu.Lock()
		if owner.terminal != nil {
			owner.mu.Unlock()
			return
		}
		var request *closedSourceWrite
		control := len(owner.controls) > 0
		if control {
			request, owner.controls = owner.controls[0], owner.controls[1:]
		} else if len(owner.data) > 0 {
			request, owner.data = owner.data[0], owner.data[1:]
		}
		if request == nil {
			changed := owner.changed
			owner.mu.Unlock()
			<-changed
			continue
		}
		owner.active = request
		owner.signalLocked()
		deadline := request.lane.writeEnd
		if !request.end.IsZero() {
			deadline = request.end
		}
		err := request.lane.writeErrorLocked(request.frame.Kind == closedFrameClose)
		if err == nil && !time.Now().Before(deadline) {
			err = os.ErrDeadlineExceeded
		}
		if err == nil {
			owner.transferred += uint64(16 + len(request.frame.Body))
			if owner.transferred > 32<<20 && (owner.chargeTerminal || request.frame.Kind != closedFrameClose) {
				err = errors.New("closed source parent byte reserve exhausted")
			} else {
				err = owner.parent.SetWriteDeadline(deadline)
			}
		}
		request.attempted = err == nil
		if request.attempted {
			request.lane.emissions++
			if request.frame.Kind != closedFrameCredit {
				request.lane.closePayloadEmissions++
			}
		}
		owner.mu.Unlock()
		attempted := request.attempted
		var before uint64
		var busy bool
		if owner.framing != nil && request.frame.Kind == closedFrameClose {
			before, busy, _ = owner.framing.writeWitness()
		}
		var parentBefore uint64
		var parentBusy bool
		parentWitness := owner.retainClosedRead && owner.framedParent != nil && (request.frame.Kind == closedFrameCredit || request.frame.Kind == closedFrameClose)
		if parentWitness {
			if request.frame.Kind == closedFrameClose {
				parentBefore, parentBusy, _ = owner.framedParent.closeWriteWitness()
			} else {
				parentBefore, parentBusy, _ = owner.framedParent.writeWitness()
			}
		}
		if attempted {
			err = WriteClosedLaneFrame(owner.parent, request.frame)
		}
		unwritten := false
		if err != nil && attempted && parentWitness {
			after, active, clean := owner.framedParent.writeWitness()
			if request.frame.Kind == closedFrameClose {
				after, active, clean = owner.framedParent.closeWriteWitness()
			}
			if parentBefore == after && (!parentBusy || request.frame.Kind == closedFrameCredit) && !active && clean {
				if request.frame.Kind == closedFrameCredit {
					// A previously counted outer write may have completed. No new
					// physical frame began, and no failed write passed the witness.
					// Keep the reader alive to consume its already accepted tail.
					err = nil
				} else {
					// Local CLOSE cannot emit on this already retired outer lane.
					// Its owner joins that retirement below; no partial frame is forgiven.
					unwritten = true
				}
			}
		}
		if err != nil && attempted && owner.framing != nil && request.frame.Kind == closedFrameClose {
			after, active, clean := owner.framing.writeWitness()
			unwritten = before == after && !busy && !active && clean
		}
		owner.mu.Lock()
		// A committed whole-parent stop owns concurrent write completion; an earlier
		// published failure is never replaced by that stop.
		if err != nil && owner.terminal == errClosedSourceStopped {
			err = errors.Join(errClosedSourceStopped, err)
		}
		if attempted && err != nil {
			request.lane.physicalWriteFailed = true
		}
		request.unwritten = unwritten
		owner.active = nil
		if request.frame.Kind == closedFrameOpen {
			request.lane.opened = err == nil
			if err != nil {
				request.lane.failure = err
			}
		}
		size := uint64(16 + len(request.frame.Body))
		owner.queued -= size
		if control {
			owner.controlsSize -= size
		}
		request.err = err
		close(request.done)
		owner.signalLocked()
		owner.mu.Unlock()
		if err != nil && attempted {
			// A failed physical write may have emitted a partial frame.
			owner.fail(err)
			return
		}
	}
}
