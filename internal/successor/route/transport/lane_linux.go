//go:build linux

package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// lane fields belong to the session lock; outbound serialization has a separate
// lock so credit waiters never hold the physical writer or a sibling lane.
type lane struct {
	s                                  *session
	id                                 uint32
	end, readEnd, writeEnd, cleanupEnd time.Time
	openEnd                            time.Time
	buffer                             []byte
	credit, receive                    uint32
	eof, closed, peerClosed            bool
	cause                              error
	changed                            chan struct{}
	writeMu                            sync.Mutex
	closeOnce                          sync.Once
	closeErr                           error
	hardEnd                            time.Time
	handshake                          bool
	handshakeBytes, uncredited         uint32
	handshakeOutput                    uint32
	openEmitted                        bool
	outputEOF                          bool
	finished                           bool
	ctx                                context.Context
	cancel                             context.CancelFunc
}

func (l *lane) signalLocked() { close(l.changed); l.changed = make(chan struct{}) }
func (l *lane) stopLocked(cause error) {
	if !l.closed {
		l.closed = true
		l.cancel()
		l.cause = cause
		l.signalLocked()
	}
}
func waitLane(changed <-chan struct{}, end time.Time) error {
	if !time.Now().Before(end) {
		return os.ErrDeadlineExceeded
	}
	timer := time.NewTimer(time.Until(end))
	defer timer.Stop()
	select {
	case <-changed:
		return nil
	case <-timer.C:
		return os.ErrDeadlineExceeded
	}
}

func (l *lane) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		l.s.mu.Lock()
		if !time.Now().Before(l.readEnd) {
			l.s.mu.Unlock()
			return 0, os.ErrDeadlineExceeded
		}
		if len(l.buffer) > 0 {
			n := copy(p, l.buffer)
			l.buffer = l.buffer[n:]
			l.s.queued -= uint64(n)
			l.s.queues.release(uint64(n))
			if len(l.buffer) == 0 && l.finished {
				delete(l.s.lanes, l.id)
			}
			// Retained payload is released only by consumption or joined owner
			// cleanup. CLOSE and outbound CREDIT never refund its reservation.
			l.receive += uint32(n)
			closed := l.closed || l.handshake
			credited := uint32(n)
			if credited <= l.uncredited {
				l.uncredited -= credited
				credited = 0
			} else {
				credited -= l.uncredited
				l.uncredited = 0
			}
			l.s.mu.Unlock()
			if !closed && credited != 0 {
				body := binary.BigEndian.AppendUint32(nil, credited)
				if err := l.s.write(l, ardp.Frame{Kind: ardp.KindCredit, Lane: l.id, Body: body}, false); err != nil {
					return n, err
				}
			}
			return n, nil
		}
		if l.closed {
			cause := l.cause
			l.s.mu.Unlock()
			if cause == nil {
				cause = io.EOF
			}
			return 0, cause
		}
		if l.eof {
			l.s.mu.Unlock()
			return 0, io.EOF
		}
		changed, end := l.changed, l.readEnd
		l.s.mu.Unlock()
		if err := waitLane(changed, end); err != nil {
			return 0, err
		}
	}
}

func (l *lane) Write(p []byte) (int, error) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	n := 0
	for len(p) > 0 {
		l.s.mu.Lock()
		if l.closed || l.outputEOF || l.s.stopped {
			l.s.mu.Unlock()
			return n, net.ErrClosed
		}
		if !time.Now().Before(l.writeEnd) {
			l.s.mu.Unlock()
			return n, os.ErrDeadlineExceeded
		}
		credit, changed, end := l.credit, l.changed, l.writeEnd
		l.s.mu.Unlock()
		if credit == 0 {
			if err := waitLane(changed, end); err != nil {
				return n, err
			}
			continue
		}
		count := min(len(p), ardp.MaximumBodySize, int(credit))
		// Copy before queueing: canceled output cannot retain caller memory.
		body := append([]byte(nil), p[:count]...)
		if err := l.s.write(l, ardp.Frame{Kind: ardp.KindBytes, Lane: l.id, Body: body}, false); err != nil {
			return n, err
		}
		n += count
		p = p[count:]
	}
	return n, nil
}

func (l *lane) closeStatus(status byte) error {
	l.closeOnce.Do(func() {
		l.s.mu.Lock()
		l.cleanupEnd = time.Now().Add(time.Second)
		peer, stopped := l.peerClosed, l.s.stopped
		emitted := l.openEmitted || l.s.open != nil
		l.stopLocked(nil)
		if l.s.active == l {
			_ = l.s.conn.SetWriteDeadline(time.Now())
		}
		l.s.mu.Unlock()
		// Joining the lane writer precedes its terminal frame. A write already
		// in physical output retains its failure through parent retirement.
		l.writeMu.Lock()
		defer l.writeMu.Unlock()
		if emitted && !peer && !stopped {
			l.closeErr = l.s.write(l, ardp.Frame{Kind: ardp.KindClose, Lane: l.id, Body: []byte{status}}, true)
		}
		if l.closeErr != nil {
			l.s.retire(l.closeErr)
		}
	})
	return l.closeErr
}
func (l *lane) Close() error         { return l.closeStatus(0) }
func (l *lane) LocalAddr() net.Addr  { return l.s.conn.LocalAddr() }
func (l *lane) RemoteAddr() net.Addr { return l.s.conn.RemoteAddr() }
func (l *lane) SetDeadline(t time.Time) error {
	return errors.Join(l.SetReadDeadline(t), l.SetWriteDeadline(t))
}
func (l *lane) SetReadDeadline(t time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if t.IsZero() || t.After(l.end) {
		t = l.end
	}
	l.readEnd = t
	l.signalLocked()
	return nil
}
func (l *lane) SetWriteDeadline(t time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if t.IsZero() || t.After(l.end) {
		t = l.end
	}
	l.writeEnd = t
	l.signalLocked()
	if l.s.active == l {
		return l.s.conn.SetWriteDeadline(minDeadline(t, l.s.activeEnd))
	}
	return nil
}

var _ net.Conn = (*lane)(nil)

// finish belongs to the work owner after its readers/writers have joined.
// Identifier floors stay in the session; retained input keeps its accounting.
func (l *lane) finish() {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.finished {
		return
	}
	l.finished = true
	l.s.live--
	l.s.queues.releaseChild()
	if len(l.buffer) == 0 {
		delete(l.s.lanes, l.id)
	}
}

func (l *lane) frameDeadline(f ardp.Frame, terminal bool) time.Time {
	if terminal {
		return minDeadline(l.cleanupEnd, l.s.end)
	}
	if f.Kind == ardp.KindCredit {
		return l.end
	}
	if f.Kind == ardp.KindOpen {
		return minDeadline(l.openEnd, l.writeEnd)
	}
	return l.writeEnd
}

func (l *lane) bound(end time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.closed || end.After(l.hardEnd) || !time.Now().Before(end) {
		return errors.New("route child horizon invalid")
	}
	l.hardEnd = end
	l.end = minDeadline(l.end, end)
	l.readEnd = minDeadline(l.readEnd, l.end)
	l.writeEnd = minDeadline(l.writeEnd, l.end)
	l.signalLocked()
	return nil
}

func (l *lane) beginRole() error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.closed || !l.handshake || !time.Now().Before(l.end) {
		return errors.New("route pending child unavailable")
	}
	l.handshake = false
	l.uncredited = uint32(len(l.buffer))
	return nil
}

func (l *lane) admit(end time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.closed || l.handshake || !time.Now().Before(l.end) || end.After(l.hardEnd) {
		return errors.New("route admitted child unavailable")
	}
	l.end = end
	l.readEnd = end
	l.writeEnd = end
	l.signalLocked()
	return nil
}

func (l *lane) CloseWrite() error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	l.s.mu.Lock()
	if l.closed || l.outputEOF {
		l.s.mu.Unlock()
		return net.ErrClosed
	}
	l.outputEOF = true
	l.s.mu.Unlock()
	return l.s.write(l, ardp.Frame{Kind: ardp.KindEOF, Lane: l.id}, false)
}
