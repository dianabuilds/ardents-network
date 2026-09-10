package node

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/route"
	"sync"
)

// The shared reader never waits for a child writer. Queued complete frames
// consume the same finite byte budget regardless of TLS record fragmentation.
// Allocation grows only with queued work, rather than reserving thousands of
// frame slots for every idle child. Route separately charges the prefix/duty.
var errClosedForwardingQueueFull = errors.New("closed forwarding reverse queue is full")

type closedForwardingQueue struct {
	mu             sync.Mutex
	changed        *sync.Cond
	frames         []route.ClosedLaneFrame
	bytes, maximum int
	closed         bool
	terminal       bool // Complete, reserved peer CLOSE; transport EOF alone is not terminal.
}

func newClosedForwardingQueue(maximum int) *closedForwardingQueue {
	queue := &closedForwardingQueue{maximum: maximum}
	queue.changed = sync.NewCond(&queue.mu)
	return queue
}

func (queue *closedForwardingQueue) push(frame route.ClosedLaneFrame, reserve func(route.ClosedLaneFrame) error) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	size := 16 + len(frame.Body)
	if queue.closed || size > queue.maximum-queue.bytes {
		return errClosedForwardingQueueFull
	}
	if reserve != nil {
		if err := reserve(frame); err != nil {
			return err
		}
	}
	queue.frames = append(queue.frames, frame)
	queue.bytes += size
	if frame.Kind == 9 {
		queue.terminal = true
	}
	queue.changed.Signal()
	return nil
}

func (queue *closedForwardingQueue) next() (route.ClosedLaneFrame, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	for len(queue.frames) == 0 && !queue.closed {
		queue.changed.Wait()
	}
	if len(queue.frames) == 0 {
		return route.ClosedLaneFrame{}, false
	}
	frame := queue.frames[0]
	queue.frames[0] = route.ClosedLaneFrame{}
	queue.frames = queue.frames[1:]
	queue.bytes -= 16 + len(frame.Body)
	if len(queue.frames) == 0 {
		queue.frames = nil
	}
	return frame, true
}

func (queue *closedForwardingQueue) close() {
	queue.mu.Lock()
	queue.closed = true
	queue.changed.Broadcast()
	queue.mu.Unlock()
}

// peerClosed remains true after draining or retiring the queue. It records the
// peer's complete terminal frame independently of the child copier's schedule.
func (queue *closedForwardingQueue) peerClosed() bool {
	if queue == nil {
		return false
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.terminal
}
