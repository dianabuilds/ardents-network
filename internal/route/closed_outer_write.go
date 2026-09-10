package route

import (
	"errors"
	"os"
	"time"
)

// Write waits for this lane's actual peer credit within its current deadline.
// Only one bounded frame enters the shared writer at a time. Waiting here
// neither reserves another window nor blocks the Carrier's other lane writers.
func (lane *ClosedOuterBridgeLane) Write(value []byte) (int, error) {
	if lane == nil || lane.lane == nil || len(value) == 0 {
		return 0, errors.New("closed outer bridge write is invalid")
	}
	inner := lane.lane
	inner.outboundWriter.Lock()
	defer inner.outboundWriter.Unlock()
	written := 0
	for len(value) != 0 {
		inner.mu.Lock()
		end := inner.currentWriteDeadline()
		if inner.dead {
			inner.mu.Unlock()
			return written, errors.New("closed outer bridge retired during write")
		}
		if !time.Now().Before(end) {
			inner.mu.Unlock()
			return written, os.ErrDeadlineExceeded
		}
		if inner.outboundCredit == 0 {
			changed := inner.outboundChanged
			inner.mu.Unlock()
			timer := time.NewTimer(time.Until(end))
			select {
			case <-changed:
			case <-timer.C:
			}
			timer.Stop()
			continue
		}
		count := min(len(value), closedLaneMaximum, int(inner.outboundCredit))
		inner.outboundCredit -= uint32(count)
		inner.mu.Unlock()
		frame := ClosedLaneFrame{Kind: closedFrameBytes, Lane: inner.id, Body: append([]byte(nil), value[:count]...)}
		if err := inner.bridge.write(frame, inner.currentWriteDeadline); err != nil {
			return written, err
		}
		value = value[count:]
		written += count
	}
	return written, nil
}

func (lane *closedOuterBridgeLane) signalOutboundLocked() {
	close(lane.outboundChanged)
	lane.outboundChanged = make(chan struct{})
}
