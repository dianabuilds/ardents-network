package channel

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"io"
	"net"
	"time"
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
	if out.err != nil && out.physical {
		result.physical = &physicalWriteFailure{kind: frame.Kind, cause: out.err}
	}
	return result
}

// physicalOutput belongs to one complete encoded write, including a deadline
// refusal before Write. Nested framing alone knows whether it emitted bytes;
// its private witnesses never leave this physical mechanism.
type physicalOutput struct {
	err                         error
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
	if out.err == io.EOF && lower != nil && cleanUnemittedRetirement(kind, before, after) {
		out.err = nil
	}
	return out
}
