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

const window = uint32(64 << 10)

// session owns one physical framing boundary and all of its borrowers. The
// owner cancels admission before interrupting I/O; Close waits for the reader
// and every child handler, retaining physical and protocol cleanup failures.
type session struct {
	conn                    net.Conn
	ctx                     context.Context
	cancel                  context.CancelFunc
	end                     time.Time
	mu                      sync.Mutex
	lanes                   map[uint32]*lane
	next, last              uint32
	live                    uint32
	queued, used, limit     uint64
	stopped                 bool
	failure                 error
	writer                  chan struct{}
	opening                 chan struct{}
	active                  *lane
	activeEnd               time.Time
	pending                 bool
	queues                  *queueBudget
	readerDone              chan struct{}
	children                sync.WaitGroup
	writes                  sync.WaitGroup
	physicalOnce, closeOnce sync.Once
	physicalErr, closeErr   error
	writeErr                error
	output                  []*frameTurn
	outbound, controlQueued uint64
	lastData                uint32
	lastControl             bool
	open                    func(context.Context, *lane, []byte) error
	check                   func() error
}

func newSession(ctx context.Context, conn net.Conn, end time.Time, limit uint64, check func() error, pending bool, queues *queueBudget, open func(context.Context, *lane, []byte) error) *session {
	child, cancel := context.WithDeadline(ctx, end)
	s := &session{conn: conn, ctx: child, cancel: cancel, end: end, lanes: make(map[uint32]*lane), next: 1,
		limit: limit, writer: make(chan struct{}, 1), opening: make(chan struct{}, 1), readerDone: make(chan struct{}), check: check, pending: pending, queues: queues, open: open}
	go s.read()
	return s
}

func (s *session) physicalClose() error {
	s.physicalOnce.Do(func() { s.physicalErr = s.conn.Close() })
	return s.physicalErr
}

func (s *session) retire(cause error) {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.failure = cause
		for _, l := range s.lanes {
			l.stopLocked(cause)
		}
	}
	s.mu.Unlock()
	s.cancel()
	_ = s.physicalClose()
}

func (s *session) Close() error {
	s.closeOnce.Do(func() {
		s.retire(nil)
		<-s.readerDone
		s.children.Wait()
		// Every selected physical write has relinquished serialization before
		// this result can authorize reservation release.
		s.writes.Wait()
		s.mu.Lock()
		s.closeErr = errors.Join(s.failure, s.physicalErr, s.writeErr)
		s.queues.release(s.queued)
		s.queued = 0
		for _, l := range s.lanes {
			l.buffer = nil
			if !l.finished {
				s.queues.releaseChild()
				l.finished = true
			}
		}
		s.mu.Unlock()
	})
	return s.closeErr
}

// joinedPhysicalFailure is read only after Close has joined all physical
// writers. Peer protocol refusal, EOF and authority cancellation remain local
// session outcomes; an owned physical write/close failure survives retirement.
func (s *session) joinedPhysicalFailure() error {
	return errors.Join(s.physicalErr, s.writeErr)
}

func (s *session) read() {
	defer close(s.readerDone)
	interruptDone := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() { defer close(interruptDone); s.retire(s.ctx.Err()) })
	defer func() {
		if !stop() {
			<-interruptDone
		}
	}()
	for {
		f, err := ardp.ReadFrame(s.conn)
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if !stopped {
				s.retire(err)
			}
			return
		}
		if s.check != nil {
			if err := s.check(); err != nil {
				s.retire(err)
				return
			}
		}
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		cost := uint64(ardp.HeaderSize + len(f.Body))
		if cost > s.limit-s.used {
			s.mu.Unlock()
			s.retire(errors.New("route byte allowance exhausted"))
			return
		}
		s.used += cost
		if f.Lane == 0 {
			s.mu.Unlock()
			s.retire(errors.New("unexpected Route parent control"))
			return
		}
		l := s.lanes[f.Lane]
		if f.Kind == ardp.KindOpen {
			if s.open == nil || l != nil || f.Lane%2 != 1 || f.Lane <= s.last || s.live >= 256 || !s.queues.child() {
				s.mu.Unlock()
				s.retire(errors.New("route child unavailable"))
				return
			}
			s.last = f.Lane
			l = s.newLaneLocked(f.Lane)
			body := append([]byte(nil), f.Body...)
			s.children.Add(1)
			s.mu.Unlock()
			go func() {
				defer s.children.Done()
				defer l.finish()
				err := s.open(l.ctx, l, body)
				status := byte(0)
				if err != nil {
					status = 1
				}
				if closeErr := l.closeStatus(status); closeErr != nil {
					s.retire(errors.Join(err, closeErr))
				}
			}()
			continue
		}
		if l == nil {
			if f.Lane%2 == 1 && ((s.open != nil && f.Lane <= s.last) || (s.open == nil && f.Lane < s.next)) {
				s.mu.Unlock()
				continue
			}
			s.mu.Unlock()
			s.retire(errors.New("unknown Route lane"))
			return
		}
		// A retired lane retains its identifier. Late input cannot acquire a
		// queue reservation or poison live siblings.
		if l.closed || !time.Now().Before(l.end) {
			l.stopLocked(os.ErrDeadlineExceeded)
			s.mu.Unlock()
			continue
		}
		switch f.Kind {
		case ardp.KindBytes:
			if l.handshake && l.handshakeBytes+l.handshakeOutput+uint32(len(f.Body)) > 4096 {
				err = errors.New("route pending TLS allowance exhausted")
			} else if l.eof || uint32(len(f.Body)) > l.receive || len(l.buffer)+len(f.Body) > int(window) || s.queued+s.outbound+uint64(len(f.Body)) > 4<<20 || !s.queues.reserve(uint64(len(f.Body))) {
				err = errors.New("route receive credit exhausted")
			} else {
				l.receive -= uint32(len(f.Body))
				if l.handshake {
					l.handshakeBytes += uint32(len(f.Body))
				}
				l.buffer = append(l.buffer, f.Body...)
				s.queued += uint64(len(f.Body))
				l.signalLocked()
			}
		case ardp.KindCredit:
			n := binary.BigEndian.Uint32(f.Body)
			if n > window-l.credit {
				err = errors.New("route credit overflow")
			} else {
				l.credit += n
				l.signalLocked()
			}
		case ardp.KindEOF:
			if l.eof {
				err = errors.New("duplicate Route EOF")
			} else {
				l.eof = true
				l.signalLocked()
			}
		case ardp.KindClose:
			l.peerClosed = true
			cause := error(nil)
			if f.Body[0] != 0 {
				cause = errors.New("route peer refused lane")
			}
			l.stopLocked(cause)
		default:
			err = errors.New("unexpected Route child frame")
		}
		s.mu.Unlock()
		if err != nil {
			s.retire(err)
			return
		}
	}
}

// write distinguishes queued work from a started physical frame. Expired
// queued work consumes neither physical output nor credit. A failed started
// frame invalidates the shared framing boundary even when Write returns zero.
func (s *session) write(l *lane, f ardp.Frame, terminal bool) error {
	raw, err := ardp.EncodeFrame(f)
	if err != nil {
		return err
	}
	release, err := s.turn(l, f, terminal, uint64(len(raw)))
	if err != nil {
		return err
	}
	defer release()
	if s.check != nil {
		if err := s.check(); err != nil {
			s.retire(err)
			return err
		}
	}
	s.mu.Lock()
	end := l.frameDeadline(f, terminal)
	if s.stopped || s.ctx.Err() != nil || (!terminal && l.closed) {
		s.mu.Unlock()
		return net.ErrClosed
	}
	if !time.Now().Before(end) {
		s.mu.Unlock()
		return os.ErrDeadlineExceeded
	}
	cost := uint64(len(raw))
	if cost > s.limit-s.used {
		s.mu.Unlock()
		return errors.New("route byte allowance exhausted")
	}
	if f.Kind == ardp.KindBytes {
		if l.handshake && uint32(len(f.Body)) > 4096-l.handshakeOutput-l.handshakeBytes {
			s.mu.Unlock()
			return errors.New("route pending TLS output allowance exhausted")
		}
		if uint32(len(f.Body)) > l.credit {
			s.mu.Unlock()
			return errors.New("route credit changed")
		}
		l.credit -= uint32(len(f.Body))
		if l.handshake {
			l.handshakeOutput += uint32(len(f.Body))
		}
	}
	s.used += cost
	s.active = l
	s.activeEnd = end
	if f.Kind == ardp.KindOpen {
		l.openEmitted = true
	}
	err = s.conn.SetWriteDeadline(end)
	s.mu.Unlock()
	if err == nil {
		for len(raw) > 0 {
			var n int
			n, err = s.conn.Write(raw)
			if err != nil {
				break
			}
			if n <= 0 || n > len(raw) {
				err = io.ErrShortWrite
				break
			}
			raw = raw[n:]
		}
	}
	s.mu.Lock()
	s.active = nil
	s.mu.Unlock()
	if err != nil {
		s.mu.Lock()
		s.writeErr = errors.Join(s.writeErr, err)
		s.mu.Unlock()
		s.retire(err)
	}
	return err
}

func (s *session) newLaneLocked(id uint32) *lane {
	l := &lane{s: s, id: id, end: s.end, readEnd: s.end, writeEnd: s.end, changed: make(chan struct{}), credit: window, receive: window}
	l.ctx, l.cancel = context.WithCancel(s.ctx)
	l.hardEnd = s.end
	l.handshake = s.pending
	if s.pending {
		l.end = minDeadline(s.end, time.Now().Add(10*time.Second))
		l.readEnd = l.end
		l.writeEnd = l.end
	}
	s.lanes[id] = l
	s.live++
	return l
}

func (s *session) openLane(ctx context.Context, body []byte) (*lane, error) {
	if ctx == nil || len(body) != 49 && len(body) != 50 {
		return nil, errors.New("route OPEN shape invalid")
	}
	opened, err := decodeOpen(body[:49])
	if err != nil {
		return nil, err
	}
	// Allocation and complete OPEN emission share one bounded operation. The
	// peer enforces monotonically increasing IDs, so a later allocation must
	// not enter physical scheduling ahead of an earlier caller.
	openEnd := minDeadline(minDeadline(opened.Deadline, s.end), time.Now().Add(10*time.Second))
	timer := time.NewTimer(time.Until(openEnd))
	defer timer.Stop()
	select {
	case s.opening <- struct{}{}:
		defer func() { <-s.opening }()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, os.ErrDeadlineExceeded
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !time.Now().Before(openEnd) {
		return nil, os.ErrDeadlineExceeded
	}
	s.mu.Lock()
	if s.stopped || s.live >= 256 || s.next == 0 || !s.queues.child() {
		s.mu.Unlock()
		return nil, errors.New("route child capacity unavailable")
	}
	id := s.next
	s.next += 2
	l := s.newLaneLocked(id)
	l.openEnd = openEnd
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = l.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := l.bound(opened.Deadline); err != nil {
		_ = l.Close()
		l.finish()
		return nil, err
	}
	l.writeMu.Lock()
	err = s.write(l, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body}, false)
	l.writeMu.Unlock()
	if err != nil {
		_ = l.Close()
		l.finish()
		return nil, errors.Join(ctx.Err(), err)
	}
	if err := ctx.Err(); err != nil {
		closeErr := l.Close()
		l.finish()
		return nil, errors.Join(err, closeErr)
	}
	return l, nil
}
