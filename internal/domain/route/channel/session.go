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

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

const Window = uint32(64 << 10)

// Session owns one physical framing boundary and all of its borrowers. The
// owner cancels admission before interrupting I/O; Close waits for the reader
// and every child handler, retaining physical and protocol cleanup failures.
type Session struct {
	conn                    net.Conn
	ctx                     context.Context
	cancel                  context.CancelFunc
	end                     time.Time
	mu                      sync.Mutex
	deadlineMu              sync.Mutex // Physical deadline effects, never state admission.
	deadlines               sync.WaitGroup
	lanes                   map[uint32]*Lane
	next, last              uint32
	live                    uint32
	reservedControls        uint32
	queued, used, limit     uint64
	stopped                 bool
	finishingRole           bool
	failure                 error
	writer                  chan struct{}
	opening                 chan struct{}
	active                  *Lane
	activeKind              uint8
	activeControl           bool
	activeEnd               time.Time
	activeDeadline          *outputDeadline
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
	closeOutcome            error
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
				s.Retire(transport.RetainedPeerReadCause(s.conn, err))
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
			if s.open == nil || l != nil || f.Lane%2 != 1 || f.Lane <= s.last {
				s.mu.Unlock()
				s.Retire(errors.New("route child unavailable"))
				return
			}
			l = s.newChildLocked(f.Lane, f.Body)
			if l == nil {
				s.mu.Unlock()
				s.Retire(errors.New("route child unavailable"))
				return
			}
			s.last = f.Lane
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
		// Retain an exact peer terminal even during local Close: it can still
		// discharge our queued terminal before physical output starts. Other
		// late traffic cannot revive the lane or acquire input capacity.
		firstPeerClose := f.Kind == ardp.KindClose && l.localClosed && !l.peerClosed
		if ((l.closed && !firstPeerClose) || !time.Now().Before(l.end)) && !(s.dedicated && f.Kind == ardp.KindClose) {
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
		var deadline *outputDeadline
		switch f.Kind {
		case ardp.KindBytes:
			if l.handshake && l.handshakeBytes+l.handshakeOutput+uint32(len(f.Body)) > 4096 {
				err = errors.New("route pending TLS allowance exhausted")
			} else if l.eof || l.pendingCredit+l.queuedCredit > l.receive || uint32(len(f.Body)) > l.receive-l.pendingCredit-l.queuedCredit || len(l.buffer)+len(f.Body) > int(Window) || s.queued+s.outbound+uint64(len(f.Body)) > 4<<20 {
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
				l.receiveCreditMetric.Decrease(uint64(len(f.Body)))
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
				l.sendCreditMetric.Increase(uint64(n))
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
			l.signalLocked() // Local retirement may already have sealed the lane.
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
			deadline = l.interruptOutputLocked()
		default:
			err = errors.New("unexpected Route child frame")
		}
		s.mu.Unlock()
		if _, interrupt := s.applyOutputDeadline(deadline); interrupt != nil {
			err = errors.Join(err, interrupt)
		}
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

func (s *Session) newLaneLocked(id uint32) *Lane {
	l := &Lane{sendCreditMetric: measurement.Track(runtimeevent.RouteSendCredit), receiveCreditMetric: measurement.Track(runtimeevent.RouteReceiveCredit), s: s, id: id, end: s.end, readEnd: s.end, writeEnd: s.end, credit: Window, receive: Window}
	l.sendCreditMetric.Increase(uint64(Window))
	l.receiveCreditMetric.Increase(uint64(Window))
	l.ctx, l.cancel = context.WithCancelCause(s.ctx)
	l.hardEnd = s.end
	l.handshake = s.pending
	if s.pending {
		l.end = MinDeadline(s.end, time.Now().Add(10*time.Second))
		l.readEnd = l.end
		l.writeEnd = l.end
	}
	s.lanes[id] = l
	s.live++
	return l
}
