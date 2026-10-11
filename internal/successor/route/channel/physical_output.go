package channel

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"io"
	"net"
)

// FrameOutput separates an operation failure from this write's owned physical
// failure. Its private provenance cannot be fabricated by relay composition.
type FrameOutput struct{ failure, physical error }

func (o FrameOutput) Err() error             { return o.failure }
func (o FrameOutput) PhysicalFailure() error { return o.physical }

// WriteFrame writes one canonical frame for a caller that already owns its
// serialization, allowance and deadline. Nested framing witnesses stay here.
func WriteFrame(conn net.Conn, frame ardp.Frame) FrameOutput {
	raw, err := ardp.EncodeFrame(frame)
	if err != nil {
		return FrameOutput{failure: err}
	}
	out := writePhysicalOutput(conn, frame.Kind, raw, nil)
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

// writePhysicalOutput is shared by the session writer and the dedicated JOIN
// relay. The caller owns serialization, accounting, deadlines and subsequent
// join. The active writer remains retained until whole-write provenance is known.
func writePhysicalOutput(conn net.Conn, kind uint8, raw []byte, initial error) physicalOutput {
	lower := lowerFramingLane(conn)
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
