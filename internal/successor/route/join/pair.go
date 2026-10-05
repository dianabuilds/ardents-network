package join

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// Each side has one synchronous frame pump. Its actual incoming frame and
// encoded outgoing frame, including the fixed RESULT, fit this reservation.
const joinFrameMemory = 2 * (ardp.HeaderSize + ardp.MaximumBodySize)

// Capacity holds one side's bounded frame memory and child position.
// Reserve it before irreversible admission spend; release only after Serve
// (or refused admission physical cleanup) has joined. Do not copy it.
type Capacity struct {
	returnMemory func()
	reserved     time.Time
	once         sync.Once
}

// ReserveCapacity claims memory from the receiving principal's shared budget.
func ReserveCapacity(queues *framing.Budget) (*Capacity, error) {
	if queues == nil {
		return nil, errors.New("route JOIN capacity unavailable")
	}
	release, err := queues.HoldChild(joinFrameMemory)
	if err != nil {
		return nil, errors.Join(errors.New("route JOIN capacity unavailable"), err)
	}
	return &Capacity{returnMemory: release, reserved: time.Now()}, nil
}

// Release returns the held position exactly once after physical join.
// A nil or unreserved capacity owns no resource.
func (c *Capacity) Release() {
	if c != nil && c.returnMemory != nil {
		c.once.Do(c.returnMemory)
	}
}

// Pairing belongs to one exact receiving duty. Each pair owns both physical
// sides until their readers and writers have returned. No Admission rule or
// shared Grant moves into this owner.
type Pairing struct {
	mu      sync.Mutex
	entries map[[32]byte]*joinPair
	record  func(error)
}

type joinPair struct {
	owner                    *Pairing
	secret, context, profile [32]byte
	sides                    [2]*joinSide
	setup                    time.Time
	matched, stopped, joined chan struct{}
	sealed                   bool
	err                      error
	interruptErr             error
	interruptOnce            sync.Once
}

type joinSide struct {
	ctx         context.Context
	conn        net.Conn
	hello       ardp.Hello
	request     ardp.JoinRequest
	check       func() error
	pair        *joinPair
	limit, used uint64
	credit      uint32
	eof         bool
	closeOnce   sync.Once
	closeErr    error
	physicalErr error
	activeKind  uint8 // guarded by pair.owner.mu while paired
}

// NewPairing binds retained physical failures to the receiving owner.
// record may be nil; otherwise it must tolerate concurrent calls and must not
// call back into this Pairing. Construction acquires no resources or authority.
func NewPairing(record func(error)) *Pairing {
	return &Pairing{record: record}
}

// Serve owns conn after valid preparation until both sides' physical work joins.
// The caller supplies the original admitted allowance and currentness check;
// Pairing neither verifies nor releases an Admission Grant. Capacity remains
// held by the caller until this method returns. The zero Pairing is usable
// without physical-failure recording; it must not be copied after use.
func (o *Pairing) Serve(ctx context.Context, conn net.Conn, hello ardp.Hello, limit uint64, check func() error, capacity *Capacity) (result error) {
	if o == nil || ctx == nil || conn == nil || capacity == nil || capacity.returnMemory == nil || hello.Purpose != ardp.PurposeDataJoin || limit < role.AdmissionWireBytes+3*ardp.HeaderSize+4096+16384+1 {
		return errors.New("route JOIN preparation absent")
	}
	s := &joinSide{ctx: ctx, conn: conn, hello: hello, check: check, limit: limit, used: role.AdmissionWireBytes, credit: framing.Window}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		o.mu.Lock()
		pair := s.pair
		o.mu.Unlock()
		if pair != nil {
			pair.stop(ctx.Err())
		} else {
			s.interrupt()
		}
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
		s.interrupt()
		result = errors.Join(result, s.closeErr)
		if s.pair == nil {
			physical := errors.Join(s.physicalErr, s.closeErr)
			result = errors.Join(result, s.physicalErr)
			if o.record != nil {
				o.record(physical)
			}
		}
		clear(s.request.Secret[:])
		clear(s.request.Context[:])
	}()
	setup := minDeadline(hello.Deadline, capacity.reserved.Add(10*time.Second))
	if err := s.deadline(setup, false); err != nil {
		return err
	}
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	if f.Kind != ardp.KindOperation || f.Lane != 1 {
		return errors.New("route JOIN requires one local lane")
	}
	s.request, err = ardp.DecodeJoinRequest(f.Body)
	clear(f.Body)
	f.Body = nil
	if err != nil {
		return err
	}
	s.used += ardp.HeaderSize + 4096
	if err := s.current(); err != nil {
		return err
	}
	if !time.Now().Before(s.request.Deadline) || s.request.Deadline.After(hello.Deadline) {
		return errors.Join(errors.New("route JOIN setup deadline invalid"), s.refuse())
	}
	setup = minDeadline(setup, s.request.Deadline)
	o.mu.Lock()
	if err := ctx.Err(); err != nil {
		o.mu.Unlock()
		return err
	}
	if !time.Now().Before(setup) {
		o.mu.Unlock()
		return context.DeadlineExceeded
	}
	if o.entries == nil {
		o.entries = make(map[[32]byte]*joinPair)
	}
	p := o.entries[s.request.Secret]
	if p != nil {
		if p.sealed || p.sides[0] != nil && p.sides[1] != nil || p.context != s.request.Context || p.profile != hello.ProfileDigest || p.sides[s.request.Side-1] != nil || !time.Now().Before(p.setup) {
			o.mu.Unlock()
			return errors.Join(errors.New("route JOIN counterpart unavailable"), s.refuse())
		}
	} else {
		p = &joinPair{owner: o, secret: s.request.Secret, context: s.request.Context, profile: hello.ProfileDigest, setup: setup, matched: make(chan struct{}), stopped: make(chan struct{}), joined: make(chan struct{})}
		o.entries[p.secret] = p
	}
	s.pair = p
	p.sides[s.request.Side-1] = s
	paired := p.sides[0] != nil && p.sides[1] != nil
	if paired {
		p.setup = minDeadline(p.setup, setup)
		close(p.matched)
	}
	o.mu.Unlock()
	if !paired {
		go p.run()
	}
	<-p.joined
	return p.err
}

func (s *joinSide) current() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if !time.Now().Before(s.hello.Deadline) {
		return context.DeadlineExceeded
	}
	if s.check != nil {
		if err := s.check(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *joinSide) interrupt() {
	s.closeOnce.Do(func() { s.closeErr = errors.Join(s.conn.SetDeadline(time.Now()), s.conn.Close()) })
}

func (p *joinPair) stop(cause error) {
	p.owner.mu.Lock()
	if !p.sealed {
		p.sealed = true
		p.err = cause
		close(p.stopped)
	}
	sides := p.sides
	p.owner.mu.Unlock()
	p.interruptOnce.Do(func() {
		// Interrupt every borrower before waiting for either side to finish.
		// Repeated/late callbacks join this same operation; they cannot mutate
		// the terminal result after the pair publishes physical completion.
		for _, s := range sides {
			if s != nil {
				if err := s.conn.SetDeadline(time.Now()); err != nil {
					p.owner.mu.Lock()
					p.err = errors.Join(p.err, err)
					p.interruptErr = errors.Join(p.interruptErr, err)
					p.owner.mu.Unlock()
				}
			}
		}
		for _, s := range sides {
			if s != nil {
				s.interrupt()
			}
		}
	})
}

func (p *joinPair) run() {
	defer func() {
		p.stop(nil)
		p.owner.mu.Lock()
		physical := p.interruptErr
		for _, s := range p.sides {
			if s != nil {
				physical = errors.Join(physical, s.physicalErr, s.closeErr)
			}
		}
		p.err = errors.Join(p.err, physical)
		delete(p.owner.entries, p.secret)
		clear(p.secret[:])
		clear(p.context[:])
		clear(p.profile[:])
		p.owner.mu.Unlock()
		if p.owner.record != nil {
			p.owner.record(physical)
		}
		close(p.joined)
	}()
	p.owner.mu.Lock()
	setup := p.setup
	p.owner.mu.Unlock()
	timer := time.NewTimer(time.Until(setup))
	defer timer.Stop()
	select {
	case <-p.matched:
	case <-p.stopped:
		return
	case <-timer.C:
		p.stop(context.DeadlineExceeded)
		return
	}
	p.owner.mu.Lock()
	sides, setup := p.sides, p.setup
	p.owner.mu.Unlock()
	completed := make(chan error, 2)
	for _, s := range sides {
		go func() {
			err := s.deadline(setup, false)
			if err == nil {
				err = s.current()
			}
			if err == nil {
				body, encodeErr := ardp.EncodeJoinResult(s.request.Nonce, 0)
				err = encodeErr
				if err == nil {
					err = s.writeFrame(ardp.Frame{Kind: ardp.KindResult, Lane: 1, Body: body})
				}
				clear(body)
			}
			if err == nil {
				err = s.current()
			}
			completed <- err
		}()
	}
	for remaining := 2; remaining > 0; {
		select {
		case err := <-completed:
			remaining--
			if err != nil {
				p.stop(err)
			}
		case <-p.stopped:
			p.stop(nil)
			for ; remaining > 0; remaining-- {
				<-completed
			}
		case <-timer.C:
			p.stop(context.DeadlineExceeded)
		}
	}
	timer.Stop()
	p.owner.mu.Lock()
	sealed := p.sealed
	p.owner.mu.Unlock()
	if sealed {
		return
	}
	for _, s := range sides {
		if err := s.deadline(s.hello.Deadline, false); err != nil {
			p.stop(err)
			return
		}
	}
	// No data reader exists until both actual result writes have returned.
	p.relay(sides)
}

// Setup and terminal cleanup retain the earlier original absolute bound.
func minDeadline(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
