package channel

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
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
	out := writePhysicalOutput(conn, frame.Kind, raw, nil, nil)
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
// join. beforeNative runs exactly once immediately before direct physical I/O.
func writePhysicalOutput(conn net.Conn, kind uint8, raw []byte, initial error, beforeNative func()) physicalOutput {
	lower := lowerFramingLane(conn)
	before := lower.retirementWitness()
	out := physicalOutput{err: initial, nested: lower != nil}
	if out.err == nil {
		for len(raw) > 0 {
			if !out.attempted && lower == nil && beforeNative != nil {
				beforeNative()
			}
			out.attempted = true
			n, err := conn.Write(raw)
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
	out.physical = out.attempted && (lower == nil || after.payload != before.payload)
	if out.err == io.EOF && lower != nil && cleanUnemittedRetirement(kind, before, after) {
		out.err = nil
	}
	return out
}
