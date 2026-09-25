//go:build linux

package route

import (
	"net"
	"os"
	"sync"
	"time"
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
