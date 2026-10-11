package channel

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
type Lane struct {
	s                                  *Session
	id                                 uint32
	end, readEnd, writeEnd, cleanupEnd time.Time
	openEnd                            time.Time
	buffer                             []byte
	credit, receive                    uint32
	eof, closed, peerClosed            bool
	cause                              error
	changed                            chan struct{}
	writeMu                            sync.Mutex
	writes                             sync.WaitGroup
	closeOnce                          sync.Once
	closeErr                           error
	hardEnd                            time.Time
	handshake                          bool
	handshakeBytes, uncredited         uint32
	handshakeOutput                    uint32
	openEmitted                        bool
	outputEOF                          bool
	finished                           bool
	afterFinish                        func()
	queueBound                         *Budget
	queuedOutput                       uint64
	queueTermination                   bool
	trafficLimit, trafficUsed          uint64
	chargeOutput                       func(uint64, bool) error
	localClosed, peerRefused           bool
	physicalAttempts, payloadAttempts  uint64
	physicalWriteFailed                bool
	refill                             *parentExchange
	ctx                                context.Context
	caller                             context.Context
	cancel                             context.CancelCauseFunc
}

func (l *Lane) signalLocked() { close(l.changed); l.changed = make(chan struct{}) }

// seal synchronously denies new effects without performing I/O. The work
// owner subsequently closes and joins this same stream before returning it.
func (l *Lane) Seal() {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	l.localClosed = true
	l.stopLocked(nil)
}

func (l *Lane) stopLocked(cause error) {
	if !l.closed {
		l.closed = true
		l.cancel(cause)
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

func (l *Lane) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		l.s.mu.Lock()
		if !time.Now().Before(l.readEnd) {
			if l.readEnd.Equal(l.end) {
				l.stopLocked(os.ErrDeadlineExceeded)
			}
			l.s.mu.Unlock()
			return 0, os.ErrDeadlineExceeded
		}
		if len(l.buffer) > 0 {
			n := copy(p, l.buffer)
			l.buffer = l.buffer[n:]
			l.s.queued -= uint64(n)
			l.s.queues.release(uint64(n))
			if l.queueBound != nil {
				l.queueBound.release(uint64(n))
			}
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
				before := l.retirementWitness()
				if err := l.s.write(l, ardp.Frame{Kind: ardp.KindCredit, Lane: l.id, Body: body}, false); err != nil {
					// The peer can send CLOSE(0) while CREDIT waits behind a
					// sibling. Already consumed bytes remain valid when this
					// unnecessary credit never started. Refusal, local retirement,
					// expiry and every physical failure still reach the reader.
					if errors.Is(err, net.ErrClosed) && cleanUnemittedRetirement(ardp.KindCredit, before, l.retirementWitness()) {
						return n, nil
					}
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
			// The selected original read timer has elapsed even if a later
			// wall reading moves backwards. Retain that exact cause for
			// downstream borrowers; a shorter caller read deadline is local.
			l.s.mu.Lock()
			if end.Equal(l.end) && end.Equal(l.readEnd) {
				l.stopLocked(os.ErrDeadlineExceeded)
			}
			l.s.mu.Unlock()
			return 0, err
		}
	}
}

func (l *Lane) Write(p []byte) (int, error) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	n := 0
	for len(p) > 0 {
		l.s.mu.Lock()
		if l.closed || l.outputEOF || l.s.stopped {
			peerEOF := l.peerClosed && !l.peerRefused && !l.localClosed && !l.s.stopped && l.s.failure == nil && !l.physicalWriteFailed
			l.s.mu.Unlock()
			if peerEOF {
				return n, io.EOF
			}
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

func (l *Lane) closeStatus(status byte) error {
	l.closeOnce.Do(func() {
		l.s.mu.Lock()
		l.localClosed = true
		emitted := l.openEmitted || l.s.open != nil || l.s.dedicated
		l.stopLocked(nil)
		interrupt := l.interruptOutputLocked()
		l.s.mu.Unlock()
		if interrupt != nil {
			// A failed deadline operation cannot leave an original writer alive
			// until its old bound or disappear from the joined physical result.
			l.s.Retire(interrupt)
			l.closeErr = interrupt
		}
		// Joining the lane writer precedes its terminal frame. A write already
		// in physical output retains its failure through parent retirement.
		l.writeMu.Lock()
		defer l.writeMu.Unlock()
		// Consumption CREDIT does not hold writeMu. All accepted lane output,
		// including queued credit, must relinquish its turn before close returns.
		l.writes.Wait()
		callerExpired := l.caller != nil && (l.caller.Err() == context.DeadlineExceeded || context.Cause(l.caller) == os.ErrDeadlineExceeded)
		sessionExpired := l.s.ctx.Err() == context.DeadlineExceeded
		l.s.mu.Lock()
		// The parent or peer can retire while this Close joins an accepted
		// writer. Its original snapshot cannot authorize another frame.
		peer, stopped := l.peerClosed, l.s.stopped
		originalEnd := l.end
		expired := l.cause == os.ErrDeadlineExceeded || !time.Now().Before(l.end) || callerExpired || sessionExpired
		if expired && l.cause == nil {
			l.cause = os.ErrDeadlineExceeded
		}
		l.s.mu.Unlock()
		// Expiry already denies this exact lane at both peers. No new frame
		// may start beyond its immutable bound; absence of a terminal write
		// is not a physical failure of the still-live framing parent.
		if emitted && !peer && !stopped && !expired {
			l.closeErr = l.s.write(l, ardp.Frame{Kind: ardp.KindClose, Lane: l.id, Body: []byte{status}}, true)
			// Parent retirement can win after the snapshot or while CLOSE is
			// queued. Only this exact unemitted CLOSE refusal is unnecessary
			// cleanup; the parent retains its original and physical failures.
			if refusal, ok := l.closeErr.(*frameRetirement); ok && refusal.lane == l && refusal.kind == ardp.KindClose {
				l.closeErr = nil
			}
			// The original bound may expire while CLOSE waits for its turn.
			// This explicit pre-output refusal is the same no-frame retirement
			// as expiry above. A shorter cleanup limit or actual I/O failure
			// remains a failed close and is never classified here.
			if refusal, ok := l.closeErr.(*frameExpiry); ok && refusal.end.Equal(originalEnd) {
				l.closeErr = nil
			}
		}
		if l.closeErr != nil && !localCapacityRefusal(l.closeErr) {
			l.s.Retire(l.closeErr)
		}
	})
	return l.closeErr
}

// The first retirement fixes one cleanup bound. Already emitted CREDIT can
// finish within that bound and its original deadline; CLOSE already carries
// the same finite bound. Payload is interrupted immediately. Called under mu.
func (l *Lane) interruptOutputLocked() error {
	if l.cleanupEnd.IsZero() {
		l.cleanupEnd = minDeadline(l.end, time.Now().Add(time.Second))
	}
	if l.s.stopped || l.s.ctx.Err() != nil || l.s.active != l || l.s.activeKind == ardp.KindClose {
		// Original cancellation already denies new effects, even before the
		// reader's retirement callback marks stopped. The parent owner interrupts
		// its physical connection. Do not reset a closed socket's deadline
		// while joining the original writer;
		// its actual late write/close failure remains with the framing owner.
		return nil
	}
	end := time.Now()
	if l.s.activeKind == ardp.KindCredit {
		l.s.activeEnd = minDeadline(l.s.activeEnd, l.cleanupEnd)
		end = l.s.activeEnd
	}
	err := l.s.conn.SetWriteDeadline(end)
	if err != nil {
		l.s.writeErr = errors.Join(l.s.writeErr, err)
	}
	return err
}
func (l *Lane) Close() error         { return l.closeStatus(0) }
func (l *Lane) LocalAddr() net.Addr  { return l.s.conn.LocalAddr() }
func (l *Lane) RemoteAddr() net.Addr { return l.s.conn.RemoteAddr() }
func (l *Lane) SetDeadline(t time.Time) error {
	return errors.Join(l.SetReadDeadline(t), l.SetWriteDeadline(t))
}

// InterruptIO denies this lane's read/write progress and interrupts its actual
// selected payload writer. Parent retirement already interrupts the physical
// stream; this operation then performs no new deadline effect. The parent still
// owns writer/reader join and retains every actual physical failure.
func (l *Lane) InterruptIO() error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	l.readEnd, l.writeEnd = time.Now(), time.Now()
	l.signalLocked()
	return l.interruptOutputLocked()
}
func (l *Lane) SetReadDeadline(t time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if t.IsZero() || t.After(l.end) {
		t = l.end
	}
	l.readEnd = t
	l.signalLocked()
	return nil
}
func (l *Lane) SetWriteDeadline(t time.Time) error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.s.stopped || l.s.ctx.Err() != nil {
		return net.ErrClosed
	}
	if t.IsZero() || t.After(l.end) {
		t = l.end
	}
	l.writeEnd = t
	l.signalLocked()
	// Receive-credit output follows the original lane/control lifetime, rather
	// than this data-write deadline. TLS half-close must not interrupt its
	// already started frame. Actual lane/parent retirement still bounds it.
	if l.s.active == l && l.s.activeKind != ardp.KindCredit {
		err := l.s.conn.SetWriteDeadline(minDeadline(t, l.s.activeEnd))
		if err != nil {
			// This exact lower owner performed the physical deadline operation.
			// Its joined result retains the failure even if an upper borrower
			// refuses before producing any lower payload.
			l.physicalWriteFailed = true
			l.s.writeErr = errors.Join(l.s.writeErr, err)
		}
		return err
	}
	return nil
}

var _ net.Conn = (*Lane)(nil)

// RetainUntilFinish transfers one physical reservation's return to this lane.
// The work owner must join its readers/writers before calling Finish; receiving
// handlers finish after their terminal write. Refusal leaves return ownership
// with the caller. The callback runs outside the framing lock.
func (l *Lane) RetainUntilFinish(release func()) error {
	if release == nil {
		return errors.New("route physical return absent")
	}
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.finished || l.afterFinish != nil {
		return errors.New("route physical return unavailable")
	}
	l.afterFinish = release
	return nil
}

// finish belongs to the work owner after its readers/writers have joined.
// Identifier floors stay in the session; retained input keeps its accounting.
func (l *Lane) Finish() {
	l.s.mu.Lock()
	if l.finished {
		l.s.mu.Unlock()
		return
	}
	l.finished = true
	if l.queueTermination {
		l.queueBound.release(ardp.HeaderSize + 1)
		l.queueTermination = false
	}
	l.s.live--
	l.s.queues.releaseChild()
	if len(l.buffer) == 0 {
		delete(l.s.lanes, l.id)
	}
	release := l.afterFinish
	l.afterFinish = nil
	if release != nil {
		l.s.finishes.Add(1)
	}
	l.s.mu.Unlock()
	if release != nil {
		defer l.s.finishes.Done()
		release()
	}
}

func (l *Lane) frameDeadline(f ardp.Frame, terminal bool) time.Time {
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

// Deadline returns the current physical bound, including pending-handshake and
// child-group shortening. It supplies no Network or admission authority.
func (l *Lane) Deadline() time.Time {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	return l.end
}

func (l *Lane) Bound(end time.Time) error {
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
	if l.s.active == l && l.s.activeEnd.After(l.end) && !l.s.stopped && l.s.ctx.Err() == nil {
		// This shortens the original lane horizon, unlike a temporary payload
		// write deadline. An already started CREDIT must obey the new original
		// bound too, while retaining its physical result until joined Close.
		l.s.activeEnd = l.end
		if err := l.s.conn.SetWriteDeadline(l.end); err != nil {
			l.physicalWriteFailed = true
			l.s.writeErr = errors.Join(l.s.writeErr, err)
			return err
		}
	}
	return nil
}

func (l *Lane) BeginRole() error {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	if l.closed || !l.handshake || !time.Now().Before(l.end) {
		return errors.New("route pending child unavailable")
	}
	l.handshake = false
	l.uncredited = uint32(len(l.buffer))
	return nil
}

func (l *Lane) Admit(end time.Time) error {
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

func (l *Lane) CloseWrite() error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	l.s.mu.Lock()
	if l.peerClosed && !l.peerRefused && !l.localClosed && !l.s.stopped && l.s.failure == nil && l.s.ctx.Err() == nil && !l.physicalWriteFailed {
		// The exact peer has terminated both directions. No further EOF may
		// be emitted on that lane; this discharges only directional cleanup,
		// while Close still joins and retains any late physical failure.
		l.s.mu.Unlock()
		return nil
	}
	if l.closed || l.outputEOF {
		l.s.mu.Unlock()
		return net.ErrClosed
	}
	l.outputEOF = true
	l.s.mu.Unlock()
	return l.s.write(l, ardp.Frame{Kind: ardp.KindEOF, Lane: l.id}, false)
}
