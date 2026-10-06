package channel

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

const Window = uint32(64 << 10)

// replaceRemaining commits a verified refill against the usage witnessed after
// its ADMIT debit. Traffic charged during verification remains charged against
// the replacement reserve; cumulative usage and child credit never reset.
func (s *Session) replaceRemaining(witness, remaining uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.dedicated || !time.Now().Before(s.end) || remaining == 0 ||
		witness > s.used || s.used > s.limit || witness > math.MaxUint64-remaining || s.used-witness > remaining {
		return errors.New("route replenishment transition unavailable")
	}
	s.limit = witness + remaining
	return nil
}

// Session owns one physical framing boundary and all of its borrowers. The
// owner cancels admission before interrupting I/O; Close waits for the reader
// and every child handler, retaining physical and protocol cleanup failures.
type Session struct {
	conn                    net.Conn
	ctx                     context.Context
	cancel                  context.CancelFunc
	end                     time.Time
	mu                      sync.Mutex
	lanes                   map[uint32]*Lane
	next, last              uint32
	live                    uint32
	queued, used, limit     uint64
	stopped                 bool
	finishingRole           bool
	failure                 error
	writer                  chan struct{}
	opening                 chan struct{}
	active                  *Lane
	activeKind              uint8
	activeEnd               time.Time
	pending                 bool
	dedicated               bool
	queues                  *Budget
	readerDone              chan struct{}
	readerOnce              sync.Once
	children                sync.WaitGroup
	writes                  sync.WaitGroup
	finishes                sync.WaitGroup
	physicalOnce, closeOnce sync.Once
	physicalErr, closeErr   error
	writeErr                error
	output                  []*frameTurn
	outbound, controlQueued uint64
	lastData                uint32
	lastControl             bool
	open                    func(context.Context, *Lane, []byte) error
	prepareOpen             func(*Lane, []byte) error
	check                   func() error
	parentControl           func(context.Context, ardp.Frame, uint64, uint64) error
	chargeOutput            func(uint64) error
	exchange                *parentExchange
}

func New(ctx context.Context, conn net.Conn, end time.Time, limit uint64, check func() error, pending bool, queues *Budget, open func(context.Context, *Lane, []byte) error) *Session {
	s := Prepare(ctx, conn, end, limit, check, pending, queues, Handlers{Open: open})
	s.Start()
	return s
}

// Start transfers the sole reader only after its owning handshake has
// committed. A refused preparation joins without starting physical input.
func (s *Session) Start() {
	s.readerOnce.Do(func() {
		s.mu.Lock()
		stopped := s.stopped
		s.mu.Unlock()
		if stopped {
			close(s.readerDone)
			return
		}
		go s.read()
	})
}

// Handlers are fixed before the reader is transferred. Composition
// supplies decisions; the framing owner supplies the accounted input witness.
type Handlers struct {
	// PrepareOpen reserves finite child capacity before the sole reader can
	// retain pipelined handshake bytes. It runs outside the framing lock and
	// must not perform transport or wait for the child handler.
	PrepareOpen   func(*Lane, []byte) error
	Open          func(context.Context, *Lane, []byte) error
	ParentControl func(context.Context, ardp.Frame, uint64, uint64) error
	// Output admits each complete physical frame before debit or emission.
	// It runs under the framing lock and must perform no I/O or reenter it.
	Output func(uint64) error
}

// Prepare leaves reading stopped so a dedicated accepted lane can be
// installed atomically before any peer data arrives at the framing owner.
func Prepare(ctx context.Context, conn net.Conn, end time.Time, limit uint64, check func() error, pending bool, queues *Budget, handlers Handlers) *Session {
	child, cancel := context.WithDeadline(ctx, end)
	s := &Session{conn: conn, ctx: child, cancel: cancel, end: end, lanes: make(map[uint32]*Lane), next: 1,
		limit:         limit,
		writer:        make(chan struct{}, 1),
		opening:       make(chan struct{}, 1),
		readerDone:    make(chan struct{}),
		check:         check,
		pending:       pending,
		queues:        queues,
		open:          handlers.Open,
		prepareOpen:   handlers.PrepareOpen,
		parentControl: handlers.ParentControl,
		chargeOutput:  handlers.Output,
	}
	return s
}

func (s *Session) physicalClose() error {
	s.physicalOnce.Do(func() {
		if err := s.conn.Close(); err != nil {
			if lowerFramingLane(s.conn) != nil {
				// Closing this borrower can emit its lower lane's CLOSE. The
				// lower framing owner retains the actual physical provenance.
				s.physicalErr = err
			} else {
				s.physicalErr = &physicalCloseFailure{owner: s, cause: err}
			}
		}
	})
	return s.physicalErr
}

func (s *Session) Retire(cause error) {
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

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.Retire(nil)
		s.Start()
		<-s.readerDone
		s.children.Wait()
		// Every selected physical write has relinquished serialization before
		// this result can authorize reservation release.
		s.writes.Wait()
		s.mu.Lock()
		s.closeErr = errors.Join(s.failure, s.physicalErr, s.writeErr)
		s.queues.release(s.queued)
		s.queued = 0
		var returns []func()
		for _, l := range s.lanes {
			if l.queueBound != nil {
				l.queueBound.release(uint64(len(l.buffer)))
				if l.queueTermination {
					l.queueBound.release(ardp.HeaderSize + 1)
					l.queueTermination = false
				}
			}
			l.buffer = nil
			if !l.finished {
				s.queues.releaseChild()
				l.finished = true
			}
			if l.afterFinish != nil {
				returns = append(returns, l.afterFinish)
				l.afterFinish = nil
			}
		}
		s.mu.Unlock()
		for _, release := range returns {
			release()
		}
		// A borrower can have detached its empty lane just before parent
		// retirement. Its already started physical return must still join.
		// Every retained lane is now finished, so no later Finish can add work.
		s.finishes.Wait()
	})
	return s.closeErr
}

// PhysicalFailure is read only after Close has joined all physical
// writers. Peer protocol refusal, EOF and authority cancellation remain local
// session outcomes; an owned physical write/close failure survives retirement.
func (s *Session) PhysicalFailure() error {
	return errors.Join(s.physicalErr, s.writeErr)
}

// Live is the framing owner's admission fact. Borrowers do not inspect its
// mutex or retirement state to decide whether an existing channel is usable.
func (s *Session) Live() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.stopped && !s.finishingRole
}

// WaitTerminal joins the sole dedicated reader within the original channel
// horizon. Its caller still joins the writer and physical owner through Close.
func (s *Session) WaitTerminal(bound time.Time) error {
	if err := s.conn.SetReadDeadline(minDeadline(s.end, bound)); err != nil {
		return err
	}
	<-s.readerDone
	return nil
}

func (s *Session) HoldControl() (func(), error) { return s.queues.HoldControl() }

func (s *Session) read() {
	defer close(s.readerDone)
	interruptDone := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() { defer close(interruptDone); s.Retire(s.ctx.Err()) })
	defer func() {
		if !stop() {
			<-interruptDone
		}
	}()
	for {
		f, err := ardp.ReadFrame(s.conn)
		if err != nil {
			if s.dedicated && err == io.EOF {
				// A raw TLS/Carrier EOF is not the inner JOIN terminal. Keep
				// already accepted bytes and their reservation until consumed.
				err = io.ErrUnexpectedEOF
			}
			s.mu.Lock()
			stopped := s.stopped
			if !stopped && s.finishingRole && err == io.EOF {
				// Orderly local role completion still needs the exact lower
				// peer CLOSE. Keep that reader alive rather than aborting its
				// Carrier as soon as the reverse direction reaches EOF.
				s.failure = err
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			if !stopped {
				s.Retire(err)
			}
			return
		}
		if s.check != nil {
			if err := s.check(); err != nil {
				s.Retire(err)
				return
			}
		}
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		cost := uint64(ardp.HeaderSize + len(f.Body))
		if f.Lane == 0 && f.Kind == ardp.KindAccept && s.exchange != nil && s.exchange.reply != nil && s.exchange.charged {
			status, credit, acceptErr := ardp.DecodeAcceptFrame(f)
			refillBytes := s.exchange.remaining
			if acceptErr != nil || status != 0 || credit != Window || s.exchange.caller == nil || s.exchange.caller.Err() != nil ||
				s.exchange.witness > s.used || s.exchange.witness > math.MaxUint64-refillBytes {
				s.mu.Unlock()
				s.Retire(errors.Join(errors.New("route refill acknowledgement refused"), acceptErr))
				return
			}
			// Replacement precedes the ACK debit; concurrent debits remain used.
			s.limit = s.exchange.witness + refillBytes
		}
		if cost > s.limit-s.used {
			s.mu.Unlock()
			s.Retire(errors.New("route byte allowance exhausted"))
			return
		}
		s.used += cost
		if s.dedicated && f.Lane != 1 {
			s.mu.Unlock()
			s.Retire(errors.New("unexpected dedicated JOIN lane"))
			return
		}
		if f.Lane == 0 {
			if s.exchange != nil && s.exchange.reply != nil && s.exchange.charged && f.Kind == ardp.KindAccept {
				reply := s.exchange.reply
				s.exchange.reply = nil
				reply <- f
				s.mu.Unlock()
				continue
			}
			remaining := s.limit - s.used
			if s.parentControl != nil && s.exchange == nil && f.Kind == ardp.KindAdmit && len(f.Body) == 355 && f.Body[0] == 2 && remaining != 0 {
				s.exchange = &parentExchange{charged: true, witness: s.used}
				witness := s.used
				s.children.Add(1)
				s.mu.Unlock()
				go func() {
					defer s.children.Done()
					defer clear(f.Body)
					err := s.parentControl(s.ctx, f, witness, remaining)
					if err != nil {
						s.Retire(err)
					}
					s.mu.Lock()
					s.exchange = nil
					s.mu.Unlock()
				}()
				continue
			}
			s.mu.Unlock()
			s.Retire(errors.New("unexpected Route parent control"))
			return
		}
		l := s.lanes[f.Lane]
		if f.Kind == ardp.KindOpen {
			if s.open == nil || l != nil || f.Lane%2 != 1 || f.Lane <= s.last || s.live >= 256 || !s.queues.child() {
				s.mu.Unlock()
				s.Retire(errors.New("route child unavailable"))
				return
			}
			s.last = f.Lane
			l = s.newLaneLocked(f.Lane)
			l.trafficUsed = cost
			body := append([]byte(nil), f.Body...)
			s.children.Add(1)
			s.mu.Unlock()
			var prepared error
			if s.prepareOpen != nil {
				prepared = s.prepareOpen(l, body)
				if prepared != nil {
					l.Seal()
				}
			}
			go func() {
				defer s.children.Done()
				defer l.Finish()
				err := prepared
				if err == nil {
					err = s.open(l.ctx, l, body)
				}
				status := byte(0)
				if err != nil {
					status = 1
				}
				if closeErr := l.closeStatus(status); closeErr != nil && !localCapacityRefusal(closeErr) {
					s.Retire(errors.Join(err, closeErr))
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
			s.Retire(errors.New("unknown Route lane"))
			return
		}
		// A retired lane retains its identifier. Late input cannot acquire a
		// queue reservation or poison live siblings.
		if (l.closed || !time.Now().Before(l.end)) && !(s.dedicated && f.Kind == ardp.KindClose) {
			l.stopLocked(os.ErrDeadlineExceeded)
			s.mu.Unlock()
			continue
		}
		if l.trafficLimit != 0 && f.Kind != ardp.KindClose {
			if cost > l.trafficLimit-l.trafficUsed {
				l.stopLocked(errors.New("route child byte allowance exhausted"))
				s.mu.Unlock()
				continue
			}
			l.trafficUsed += cost
		}
		switch f.Kind {
		case ardp.KindBytes:
			if l.handshake && l.handshakeBytes+l.handshakeOutput+uint32(len(f.Body)) > 4096 {
				err = errors.New("route pending TLS allowance exhausted")
			} else if l.eof || uint32(len(f.Body)) > l.receive || len(l.buffer)+len(f.Body) > int(Window) || s.queued+s.outbound+uint64(len(f.Body)) > 4<<20 {
				err = errors.New("route receive credit exhausted")
			} else if l.queueBound != nil && !l.queueBound.reserve(uint64(len(f.Body))) {
				// Exhausting this child group's smaller queue denies only this
				// child. Its reserved terminal capacity still permits CLOSE.
				l.stopLocked(errors.New("route child queue exhausted"))
			} else if !s.queues.reserve(uint64(len(f.Body))) {
				if l.queueBound != nil {
					l.queueBound.release(uint64(len(f.Body)))
				}
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
			if n > Window-l.credit {
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
			l.peerRefused = f.Body[0] != 0
			cause := error(nil)
			if f.Body[0] != 0 {
				cause = errors.New("route peer refused lane")
				if s.dedicated {
					// This sole lane is the whole JOIN channel, so its refusal
					// also remains the channel's joined terminal outcome.
					err = cause
				}
			}
			l.stopLocked(cause)
			if interrupt := l.interruptOutputLocked(); interrupt != nil {
				err = interrupt
			}
		default:
			err = errors.New("unexpected Route child frame")
		}
		s.mu.Unlock()
		if err != nil {
			s.Retire(err)
			return
		}
		if s.dedicated && f.Kind == ardp.KindClose {
			// The authenticated inner terminal ends this reader. The stream
			// owner still joins writers and drains or releases retained input.
			return
		}
	}
}

// write distinguishes queued work from a started physical frame. Expired
// queued work consumes neither physical output nor credit. A failed started
// frame invalidates the shared framing boundary even when Write returns zero.
func (s *Session) write(l *Lane, f ardp.Frame, terminal bool) error {
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
			s.Retire(err)
			return err
		}
	}
	s.mu.Lock()
	end := l.frameDeadline(f, terminal)
	if s.stopped || s.finishingRole || s.ctx.Err() != nil || (!terminal && l.closed) {
		s.mu.Unlock()
		return &frameRetirement{lane: l, kind: f.Kind}
	}
	if !time.Now().Before(end) {
		s.mu.Unlock()
		return &frameExpiry{end: end}
	}
	if f.Kind == ardp.KindAdmit && f.Lane == 0 && l.caller != nil {
		// Currentness observation can finish after the original operation is
		// canceled, before its cancellation callback closes the control lane.
		// Refuse synchronously before capacity, debit or physical output.
		if err := l.caller.Err(); err != nil {
			s.mu.Unlock()
			return err
		}
	}
	cost := uint64(len(raw))
	if cost > s.limit-s.used {
		s.mu.Unlock()
		return errors.New("route byte allowance exhausted")
	}
	if f.Kind == ardp.KindAdmit && f.Lane == 0 && cost == s.limit-s.used {
		s.mu.Unlock()
		return errors.New("route refill requires positive original reserve")
	}
	if f.Kind == ardp.KindAdmit && f.Lane == 0 && l.refill != nil && l.refill.requiresHold {
		needed := l.refill.remaining - (s.limit - s.used - cost)
		if needed > l.refill.held {
			s.mu.Unlock()
			return &refillCapacityDelta{additional: needed - l.refill.held}
		}
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
	}
	if l.trafficLimit != 0 && !(terminal && f.Kind == ardp.KindClose) && cost > l.trafficLimit-l.trafficUsed {
		cause := errors.New("route child byte allowance exhausted")
		l.stopLocked(cause)
		s.mu.Unlock()
		return &frameCapacityRefusal{cause: cause}
	}
	if s.chargeOutput != nil {
		if err := s.chargeOutput(cost); err != nil {
			l.stopLocked(err)
			s.mu.Unlock()
			return &frameCapacityRefusal{cause: err}
		}
	}
	if l.chargeOutput != nil {
		if err := l.chargeOutput(cost, terminal && f.Kind == ardp.KindClose); err != nil {
			l.stopLocked(err)
			s.mu.Unlock()
			return &frameCapacityRefusal{cause: err}
		}
	}
	if l.trafficLimit != 0 && !(terminal && f.Kind == ardp.KindClose) {
		l.trafficUsed += cost
	}
	if f.Kind == ardp.KindBytes {
		l.credit -= uint32(len(f.Body))
		if l.handshake {
			l.handshakeOutput += uint32(len(f.Body))
		}
	}
	s.used += cost
	if f.Kind == ardp.KindAdmit && f.Lane == 0 {
		if l.refill != nil {
			l.refill.witness = s.used
			l.refill.charged = true
		}
	}
	s.active = l
	s.activeKind = f.Kind
	s.activeEnd = end
	if f.Kind == ardp.KindOpen {
		l.openEmitted = true
	}
	err = s.conn.SetWriteDeadline(end)
	s.mu.Unlock()
	output := writePhysicalOutput(s.conn, f.Kind, raw, err, func() {
		s.mu.Lock()
		l.physicalAttempts++
		if f.Kind != ardp.KindCredit {
			l.payloadAttempts++
		}
		s.mu.Unlock()
	})
	err = output.err
	if output.nested && output.physical {
		s.mu.Lock()
		l.physicalAttempts++
		if f.Kind != ardp.KindCredit {
			l.payloadAttempts++
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.active = nil
	s.mu.Unlock()
	if err != nil {
		s.mu.Lock()
		if output.physical {
			l.physicalWriteFailed = true
			err = &physicalWriteFailure{owner: s, kind: f.Kind, cause: err}
			s.writeErr = errors.Join(s.writeErr, err)
		} else if !output.attempted && !output.nested {
			// A physical deadline operation failed before Write; preserve
			// that owned I/O failure without claiming a started frame. A nested
			// lower deadline refusal belongs to its lower framing owner.
			l.physicalWriteFailed = true
			s.writeErr = errors.Join(s.writeErr, err)
		}
		s.mu.Unlock()
		// A lower refusal/local closure with no new lower output remains this
		// channel's failed terminal result, not a fabricated physical failure
		// of its receiving owner. Actual lower failures stay with that owner.
		s.Retire(err)
	}
	return err
}

func (s *Session) newLaneLocked(id uint32) *Lane {
	l := &Lane{s: s, id: id, end: s.end, readEnd: s.end, writeEnd: s.end, changed: make(chan struct{}), credit: Window, receive: Window}
	l.ctx, l.cancel = context.WithCancelCause(s.ctx)
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

func (s *Session) Open(ctx, caller context.Context, body []byte) (*Lane, error) {
	if ctx == nil || caller == nil || len(body) != 49 && len(body) != 50 {
		return nil, errors.New("route OPEN shape invalid")
	}
	var opened ardp.Open
	var err error
	if len(body) == 50 {
		var envelope ardp.NodeOpen
		envelope, err = ardp.DecodeNodeOpen(body)
		opened = envelope.Recipient
	} else {
		opened, err = ardp.DecodeOpen(body)
	}
	if err != nil {
		return nil, err
	}
	if !time.Now().Before(opened.Deadline) {
		return nil, errors.New("route OPEN facts invalid")
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
	case <-caller.Done():
		return nil, caller.Err()
	case <-timer.C:
		return nil, os.ErrDeadlineExceeded
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := caller.Err(); err != nil {
		return nil, err
	}
	if !time.Now().Before(openEnd) {
		return nil, os.ErrDeadlineExceeded
	}
	s.mu.Lock()
	if s.stopped || s.finishingRole || s.live >= 256 || s.next == 0 || !s.queues.child() {
		s.mu.Unlock()
		return nil, errors.New("route child capacity unavailable")
	}
	id := s.next
	s.next += 2
	l := s.newLaneLocked(id)
	// Cancellation of the physical child may lose the original deadline reason.
	// Retain the actual caller so retirement cannot resurrect expired output.
	l.caller = caller
	l.openEnd = openEnd
	s.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = l.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := l.Bound(opened.Deadline); err != nil {
		_ = l.Close()
		l.Finish()
		return nil, err
	}
	l.writeMu.Lock()
	err = s.write(l, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body}, false)
	l.writeMu.Unlock()
	if err != nil {
		_ = l.Close()
		l.Finish()
		return nil, errors.Join(ctx.Err(), caller.Err(), err)
	}
	if err := errors.Join(ctx.Err(), caller.Err()); err != nil {
		closeErr := l.Close()
		l.Finish()
		return nil, errors.Join(err, closeErr)
	}
	return l, nil
}

func minDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// Done signals sole-reader retirement. Close still joins every writer and child.
func (s *Session) Done() <-chan struct{} { return s.readerDone }
