package channel

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

// One exact selected frame owns its deadline across lower I/O and interruption.
// Session.mu guards its desired fields; deadlineMu serializes physical effects.
type outputDeadline struct {
	lane    *Lane
	end     time.Time
	cutoff  time.Time
	control bool
}

// Caller holds Session.mu. Active output remains retained by the writer turn;
// Close joins writers before waiting for these original deadline users.
func (l *Lane) retainOutputDeadlineLocked() *outputDeadline {
	deadline := l.s.activeDeadline
	l.s.deadlines.Add(1)
	l.deadlines.Add(1)
	return deadline
}

func (s *Session) applyOutputDeadline(deadline *outputDeadline) (performed bool, result error) {
	if deadline == nil {
		return false, nil
	}
	defer s.deadlines.Done()
	defer deadline.lane.deadlines.Done()
	s.deadlineMu.Lock()
	defer s.deadlineMu.Unlock()
	for {
		s.mu.Lock()
		if s.stopped || s.ctx.Err() != nil || s.activeDeadline != deadline {
			s.mu.Unlock()
			return performed, result
		}
		end, control := deadline.end, deadline.control
		s.mu.Unlock()
		err := boundPhysicalControl(s.conn, end, control)
		performed = true
		s.mu.Lock()
		deadline.lane.retainDeadlineFailureLocked(err)
		changed := s.activeDeadline == deadline && (deadline.end != end || deadline.control != control)
		s.mu.Unlock()
		if err != nil || !changed {
			return performed, err
		}
		// A winning interruption can change the bound during physical I/O.
		// Apply it before relinquishing this exact frame's deadline turn.
	}
}

// FrameOutput separates an operation failure from its owned physical
// failure. Its private provenance cannot be fabricated by relay composition.
type FrameOutput struct{ failure, physical error }

func (o FrameOutput) Err() error             { return o.failure }
func (o FrameOutput) PhysicalFailure() error { return o.physical }

// SetFrameDeadline changes the original framing connection's deadline and
// retains its work result separately from an owned physical failure. A nested
// lane retains any actual lower failure itself; its local refusal cannot mint
// another physical effect at this borrowing boundary. writeOnly selects the
// net.Conn write deadline instead of changing both directions.
func SetFrameDeadline(conn net.Conn, end time.Time, writeOnly bool) FrameOutput {
	var err error
	if writeOnly {
		err = conn.SetWriteDeadline(end)
	} else {
		err = conn.SetDeadline(end)
	}
	return frameDeadlineResult(conn, err)
}

// BoundControlOutput carries an earlier cleanup bound through the exact lower
// framing lane of an already selected control. An ordinary payload deadline
// cannot interrupt that control, but its owning retirement must still bound
// the real physical writer. A retired lower owner needs no new deadline effect.
func BoundControlOutput(conn net.Conn, end time.Time) FrameOutput {
	return frameDeadlineResult(conn, boundPhysicalControl(conn, end, true))
}

func frameDeadlineResult(conn net.Conn, err error) FrameOutput {
	result := FrameOutput{failure: err}
	if err != nil && lowerFramingLane(conn) == nil {
		result.physical = err
	}
	return result
}

// WriteFrame writes one canonical frame for a caller that already owns its
// serialization, allowance and deadline. Nested framing witnesses stay here.
func WriteFrame(conn net.Conn, frame ardp.Frame) FrameOutput {
	raw, err := ardp.EncodeFrame(frame)
	if err != nil {
		return FrameOutput{failure: err}
	}
	out := writePhysicalOutput(conn, frame.Kind, raw, nil, frame.Kind == ardp.KindCredit)
	result := FrameOutput{failure: out.err}
	if out.failure != nil {
		result.physical = out.failure
		if !out.nested {
			result.physical = &physicalWriteFailure{kind: frame.Kind, cause: out.failure}
		}
	}
	return result
}

// WriteJoinRequest emits the one holder setup request on its unpublished TLS
// lane. Before physical admission cancellation denies output. After admission
// the exact selected frame joins within its original cleanup bound so local
// cancellation cannot abort a shared TLS record and retire live siblings.
func WriteJoinRequest(conn net.Conn, request ardp.JoinRequest) error {
	lane := lowerFramingLane(conn)
	if lane == nil {
		return errors.New("route JOIN request requires its original framing lane")
	}
	body, err := ardp.EncodeJoinRequest(request)
	if err != nil {
		return err
	}
	defer clear(body)
	raw, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: body})
	if err != nil {
		return err
	}
	defer clear(raw)
	lane.s.mu.Lock()
	lane.joinRequestOutput = true
	lane.s.mu.Unlock()
	defer func() {
		lane.s.mu.Lock()
		lane.joinRequestOutput = false
		lane.s.mu.Unlock()
	}()
	return writePhysicalOutput(conn, ardp.KindOperation, raw, nil, false).err
}

// physicalOutput belongs to one complete encoded write, including a deadline
// refusal before Write. Nested framing alone knows whether it emitted bytes;
// its private witnesses never leave this physical mechanism.
type physicalOutput struct {
	err, failure                error
	attempted, physical, nested bool
}

func boundPhysicalControl(conn net.Conn, end time.Time, control bool) error {
	if control {
		if lower := lowerFramingLane(conn); lower != nil {
			return lower.boundControlOutput(end)
		}
	}
	return conn.SetWriteDeadline(end)
}

// writePhysicalOutput is shared by the session writer and the dedicated JOIN
// relay. The caller owns serialization, accounting, deadlines and subsequent
// join. The active writer remains retained until whole-write provenance is known.
func writePhysicalOutput(conn net.Conn, kind uint8, raw []byte, initial error, control bool) physicalOutput {
	lower := lowerFramingLane(conn)
	if control && lower != nil {
		// TLS carries this already selected CREDIT as lower BYTES. Preserve
		// its original bounded retirement through each framing layer without
		// inspecting ciphertext or granting another frame/credit effect. The
		// sole owning writer holds this borrow only through its whole output.
		lower.s.mu.Lock()
		lower.controlOutput++
		lower.s.mu.Unlock()
		defer func() {
			lower.s.mu.Lock()
			lower.controlOutput--
			lower.s.mu.Unlock()
		}()
	}
	before := lower.retirementWitness()
	out := physicalOutput{err: initial, nested: lower != nil}
	emitted := false
	if out.err == nil {
		for len(raw) > 0 {
			out.attempted = true
			n, err := conn.Write(raw)
			emitted = emitted || n > 0
			if err != nil {
				out.err = err
				break
			}
			if n <= 0 || n > len(raw) {
				out.err = io.ErrShortWrite
				break
			}
			raw = raw[n:]
		}
	}
	after := lower.retirementWitness()
	out.physical = out.attempted && ((lower == nil && (emitted || !transport.IsUnstartedWrite(out.err))) || (lower != nil && after.payload != before.payload))
	if out.err != nil {
		if lower == nil && out.physical {
			out.failure = out.err
		} else if lower != nil {
			// Successful lower bytes followed by lane retirement leave this
			// upper frame incomplete, but cannot invent a native cleanup
			// failure. Retain the lower owner's actual physical failure,
			// independently of its successful output count and work refusal.
			out.failure = after.failure
		}
	}
	if out.err == io.EOF && lower != nil && cleanUnemittedRetirement(kind, before, after) {
		out.err = nil
	}
	return out
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
	if s.stopped || s.finishingRole || s.ctx.Err() != nil || l.peerClosed || (!terminal && l.closed) {
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
		l.sendCreditMetric.Decrease(uint64(len(f.Body)))
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
	// An already selected directional EOF is a terminal, not payload. Like
	// selected CREDIT it must join its actual writer under bounded cleanup;
	// expiring it inside TLS can poison a healthy multiplexed parent.
	joinRequest := f.Kind == ardp.KindBytes && l.joinRequestOutput
	control := f.Kind == ardp.KindCredit || f.Kind == ardp.KindEOF || l.controlOutput != 0 || joinRequest
	s.activeControl = control
	s.activeEnd = end
	s.activeDeadline = &outputDeadline{lane: l, end: end, cutoff: end}
	deadline := l.retainOutputDeadlineLocked()
	s.mu.Unlock()
	performed, err := s.applyOutputDeadline(deadline)
	output := physicalOutput{err: err}
	refused := false
	if performed && err == nil {
		// The deadline effect can block outside state while this exact lane
		// seals. Refuse before entering TLS/Carrier Write: even an unemitted
		// TLS failure can make that shared writer permanently unusable.
		// Already selected control retains only its original cleanup bound.
		s.mu.Lock()
		requestCanceled := joinRequest && l.caller != nil && l.caller.Err() != nil
		if s.activeDeadline != deadline || s.stopped || s.finishingRole || s.ctx.Err() != nil || requestCanceled ||
			(terminal && l.peerClosed) || (!terminal && l.closed && (!control || f.Kind == ardp.KindEOF || joinRequest)) {
			output.err = &frameRetirement{lane: l, kind: f.Kind}
			refused = true
		} else if !time.Now().Before(deadline.end) {
			output.err = &frameExpiry{end: deadline.end}
			refused = true
		} else if f.Kind == ardp.KindOpen {
			// Cleanup may emit CLOSE only once this original OPEN is
			// admitted to Write, never merely because its deadline started.
			l.openEmitted = true
		} else if f.Kind == ardp.KindCredit {
			credit := binary.BigEndian.Uint32(f.Body)
			if credit > l.queuedCredit {
				output.err = errors.New("route unearned receive credit")
				refused = true
			} else {
				// The peer may use the grant before Write returns. Admit it at
				// physical output, never while consumption or queued output waits.
				l.queuedCredit -= credit
			}
		}
		s.mu.Unlock()
	}
	if performed && !refused {
		output = writePhysicalOutput(s.conn, f.Kind, raw, err, control)
	} else if !performed && err == nil {
		output.err = &frameRetirement{lane: l, kind: f.Kind}
		refused = true
	}
	err = output.err
	s.mu.Lock()
	// The selected writer stays active until native provenance and failure are
	// published together. An adapter refusal cannot mint a lower-lane attempt;
	// nested TLS still retains every preceding physical record from this write.
	if output.physical {
		l.physicalAttempts++
		if f.Kind != ardp.KindCredit {
			l.payloadAttempts++
		}
	}
	if err != nil {
		if output.failure != nil {
			l.physicalWriteFailed = true
			physical := output.failure
			if !output.nested {
				physical = &physicalWriteFailure{owner: s, kind: f.Kind, cause: physical}
				err = physical
			} else {
				err = errors.Join(err, physical)
			}
			l.physicalErr = errors.Join(l.physicalErr, physical)
			s.writeErr = errors.Join(s.writeErr, physical)
		}
	}
	s.active = nil
	s.activeDeadline = nil
	s.activeControl = false
	s.mu.Unlock()
	if err != nil && !refused {
		// A lower refusal/local closure with no new lower output remains this
		// channel's failed terminal result, not a fabricated physical failure
		// of its receiving owner. Actual lower failures stay with that owner.
		s.Retire(err)
	}
	return err
}
