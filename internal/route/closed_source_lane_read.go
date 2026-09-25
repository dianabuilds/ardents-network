//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

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
