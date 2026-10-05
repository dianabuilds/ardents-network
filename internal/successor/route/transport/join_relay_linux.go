//go:build linux

package transport

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Only an actual read interrupted by our deadline can be an expected second
// pump result. A protocol/authority refusal or an unrelated joined error is
// never normalized merely because cleanup has started.
type joinReadFailure struct{ cause error }

func (e *joinReadFailure) Error() string { return e.cause.Error() }
func (e *joinReadFailure) Unwrap() error { return e.cause }

func joinReadDeadlineOnly(err error) bool {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !joinReadDeadlineOnly(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return joinReadDeadlineOnly(wrapped.Unwrap())
	}
	return err == os.ErrDeadlineExceeded
}

func (s *joinSide) account(n uint64, terminal bool) error {
	if err := s.current(); err != nil {
		return err
	}
	p := s.pair
	if p != nil {
		p.owner.mu.Lock()
		defer p.owner.mu.Unlock()
		if p.sealed && !terminal {
			return errors.New("route JOIN ended")
		}
	}
	if s.used > s.limit || n > s.limit-s.used {
		return errors.New("route JOIN allowance exhausted")
	}
	s.used += n
	return nil
}

// Failed physical deadline operations survive the receiving owner's terminal
// result even if subsequent interruption and Close succeed. Paired failures
// use the pair lock because an interrupted writer may finish concurrently.
func (s *joinSide) deadline(end time.Time, writeOnly bool) error {
	var err error
	if writeOnly {
		err = s.conn.SetWriteDeadline(end)
	} else {
		err = s.conn.SetDeadline(end)
	}
	if err != nil {
		if s.pair == nil {
			s.physicalErr = errors.Join(s.physicalErr, err)
		} else {
			s.pair.owner.mu.Lock()
			s.pair.interruptErr = errors.Join(s.pair.interruptErr, err)
			s.pair.owner.mu.Unlock()
		}
	}
	return err
}

// A canonical unmatched request receives the same fixed-size local outcome;
// it never joins, mutates or cancels an already retained side.
func (s *joinSide) refuse() error {
	body, err := ardp.EncodeJoinResult(s.request.Nonce, 1)
	if err != nil {
		return err
	}
	defer clear(body)
	return s.writeFrame(ardp.Frame{Kind: ardp.KindResult, Lane: 1, Body: body})
}

func (s *joinSide) readFrame() (ardp.Frame, error) {
	var header [ardp.HeaderSize]byte
	if err := s.account(ardp.HeaderSize, false); err != nil {
		return ardp.Frame{}, err
	}
	if _, err := io.ReadFull(s.conn, header[:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return ardp.Frame{}, &joinReadFailure{cause: err}
	}
	if !ardp.ValidHeader(header[:]) || binary.BigEndian.Uint32(header[8:12]) != 1 {
		return ardp.Frame{}, errors.New("route JOIN frame header invalid")
	}
	length := binary.BigEndian.Uint32(header[12:16])
	if err := s.account(uint64(length), false); err != nil {
		return ardp.Frame{}, err
	}
	f := ardp.Frame{Kind: header[6], Lane: 1, Body: make([]byte, length)}
	if _, err := io.ReadFull(s.conn, f.Body); err != nil {
		return ardp.Frame{}, &joinReadFailure{cause: err}
	}
	if !ardp.ValidFrame(f) {
		return ardp.Frame{}, errors.New("route JOIN frame invalid")
	}
	return f, nil
}

// Each side has one writer: its own RESULT before the barrier, then its
// opposite pump, then joined terminal cleanup. The reservation includes both
// body and encoded frame. Track the whole TLS write, never one error branch.
func (s *joinSide) writeFrame(f ardp.Frame) error {
	if s.pair != nil {
		s.pair.owner.mu.Lock()
		s.activeKind = f.Kind
		s.pair.owner.mu.Unlock()
		defer func() {
			s.pair.owner.mu.Lock()
			s.activeKind = 0
			s.pair.owner.mu.Unlock()
		}()
	}
	if err := s.account(uint64(ardp.HeaderSize+len(f.Body)), f.Kind == ardp.KindClose); err != nil {
		return err
	}
	raw, err := ardp.EncodeFrame(f)
	if err != nil {
		return err
	}
	lower := lowerFramingLane(s.conn)
	before := lower.retirementWitness()
	attempted := false
	for len(raw) > 0 {
		attempted = true
		n, writeErr := s.conn.Write(raw)
		if writeErr != nil {
			err = writeErr
			break
		}
		if n <= 0 || n > len(raw) {
			err = io.ErrShortWrite
			break
		}
		raw = raw[n:]
	}
	after := lower.retirementWitness()
	if err == io.EOF && lower != nil && cleanUnemittedRetirement(f.Kind, before, after) {
		err = nil
	}
	if err != nil && attempted && (lower == nil || after.payload != before.payload) {
		s.physicalErr = errors.Join(s.physicalErr, &physicalWriteFailure{kind: f.Kind, cause: err})
	}
	return err
}

type joinPumpResult struct {
	err      error
	write    bool
	terminal bool
	status   byte
}

func (s *joinSide) pump(peer *joinSide) joinPumpResult {
	for {
		f, err := s.readFrame()
		if err != nil {
			return joinPumpResult{err: err}
		}
		p := s.pair
		p.owner.mu.Lock()
		if p.sealed {
			p.owner.mu.Unlock()
			return joinPumpResult{err: errors.New("route JOIN ended")}
		}
		switch f.Kind {
		case ardp.KindBytes:
			if s.eof || uint32(len(f.Body)) > s.credit {
				err = errors.New("route JOIN receive credit exceeded")
			} else {
				s.credit -= uint32(len(f.Body))
			}
		case ardp.KindCredit:
			n := binary.BigEndian.Uint32(f.Body)
			if n == 0 || n > window-peer.credit {
				err = errors.New("route JOIN credit exceeds consumption")
			} else {
				peer.credit += n
			}
		case ardp.KindEOF:
			if s.eof {
				err = errors.New("route JOIN repeated directional EOF")
			} else {
				s.eof = true
			}
		case ardp.KindClose:
			p.owner.mu.Unlock()
			return joinPumpResult{terminal: true, status: f.Body[0]}
		default:
			err = errors.New("route JOIN operation after activation")
		}
		p.owner.mu.Unlock()
		if err != nil {
			return joinPumpResult{err: err}
		}
		if err := peer.writeFrame(f); err != nil {
			return joinPumpResult{err: err, write: true}
		}
		// A selected write may finish during terminal retirement. Do not
		// start another read after that exact pair has synchronously sealed.
		p.owner.mu.Lock()
		sealed := p.sealed
		p.owner.mu.Unlock()
		if sealed {
			return joinPumpResult{}
		}
	}
}

// A verified peer terminal interrupts input and payload immediately, but an
// already selected CREDIT retains the earlier original/one-second write bound.
// No observer, physical operation or wait runs under the pair lock.
func (s *joinSide) interruptRelay() error {
	s.pair.owner.mu.Lock()
	credit := s.activeKind == ardp.KindCredit
	s.pair.owner.mu.Unlock()
	if !credit {
		return s.deadline(time.Now(), false)
	}
	now := time.Now()
	readErr := s.conn.SetReadDeadline(now)
	if readErr != nil {
		s.pair.owner.mu.Lock()
		s.pair.interruptErr = errors.Join(s.pair.interruptErr, readErr)
		s.pair.owner.mu.Unlock()
	}
	return errors.Join(readErr, s.deadline(minDeadline(s.hello.Deadline, now.Add(time.Second)), true))
}

func (p *joinPair) relay(sides [2]*joinSide) {
	completed := make(chan joinPumpResult, 2)
	go func() { completed <- sides[0].pump(sides[1]) }()
	go func() { completed <- sides[1].pump(sides[0]) }()
	var first joinPumpResult
	select {
	case first = <-completed:
	case <-p.stopped:
		p.stop(nil)
		<-completed
		<-completed
		return
	}
	if !first.terminal {
		p.stop(first.err)
		<-completed
		return
	}
	p.owner.mu.Lock()
	if !p.sealed {
		p.sealed = true
		if first.status != 0 {
			p.err = errors.New("route JOIN peer refused")
		}
		close(p.stopped)
	}
	p.owner.mu.Unlock()
	// Seal before interrupting both pumps. A directional EOF never reaches
	// this path. Physical write failure remains visible even during cleanup.
	interrupted := true
	for _, s := range sides {
		if err := s.interruptRelay(); err != nil {
			interrupted = false
			p.owner.mu.Lock()
			p.err = errors.Join(p.err, err)
			p.owner.mu.Unlock()
		}
	}
	second := <-completed
	secondErr := second.err
	if second.terminal && second.status != 0 {
		secondErr = errors.Join(secondErr, errors.New("route JOIN opposite peer refused"))
	}
	if read, ok := secondErr.(*joinReadFailure); ok && interrupted && joinReadDeadlineOnly(read.cause) {
		secondErr = nil
	}
	if secondErr != nil {
		p.owner.mu.Lock()
		p.err = errors.Join(p.err, secondErr)
		p.owner.mu.Unlock()
	}
	if sides[0].physicalErr != nil || sides[1].physicalErr != nil || secondErr != nil {
		// A damaged framing boundary admits no later encoded output. Retire
		// physically, preserving the already joined failure, instead of trying
		// to manufacture a terminal success after a partial frame.
		return
	}
	// Only joined writers may emit terminal frames. Their old absolute bounds
	// still apply; no failed started write becomes a clean terminal result.
	for _, s := range sides {
		end := minDeadline(s.hello.Deadline, time.Now().Add(time.Second))
		err := s.deadline(end, true)
		if err == nil {
			err = s.writeFrame(ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{first.status}})
		}
		if err != nil {
			p.owner.mu.Lock()
			p.err = errors.Join(p.err, err)
			p.owner.mu.Unlock()
			// No later side may receive successful terminal output after this
			// physical boundary failed during terminal emission itself.
			break
		}
	}
}
