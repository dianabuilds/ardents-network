package channel

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These controls supply physical scheduling, never successful role authority.
// The actual incoming OPEN allocates two siblings before either sends data.
func constrainedIncomingChildren(t *testing.T, maximum uint64, output func(uint64, bool) error) (*Session, *Lane, *Lane, *lifecycleConn) {
	t.Helper()
	physical := newLifecycleConn(false)
	var input []byte
	for _, id := range []uint32{1, 3} {
		raw, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: make([]byte, 50)})
		if err != nil {
			t.Fatal(err)
		}
		input = append(input, raw...)
	}
	physical.input = bytes.NewReader(input)
	ready := make(chan *Lane, 2)
	s := Prepare(t.Context(), physical, time.Now().Add(10*time.Second), 1<<20, nil, true, NewBudget(1<<20), Handlers{
		PrepareOpen: func(l *Lane, _ []byte) error {
			if l.id == 1 {
				return l.ConstrainTraffic(maximum, output)
			}
			return nil
		},
		Open: func(ctx context.Context, l *Lane, _ []byte) error {
			ready <- l
			<-ctx.Done()
			return context.Cause(ctx)
		},
	})
	s.Start()
	t.Cleanup(func() { s.Retire(nil); _ = s.Close() })
	first, sibling := <-ready, <-ready
	if first.id != 1 {
		first, sibling = sibling, first
	}
	if err := first.BeginRole(); err != nil {
		t.Fatal(err)
	}
	if err := sibling.BeginRole(); err != nil {
		t.Fatal(err)
	}
	return s, first, sibling, physical
}

func TestChildTrafficIncludesOPENHeadersAndPreheldTerminals(t *testing.T) {
	var dataCost, terminalCost uint64
	// OPEN=66, two CLOSE reserves=34, two one-byte BYTES frames=34.
	s, limited, ordinary, physical := constrainedIncomingChildren(t, 134, func(cost uint64, terminal bool) error {
		if terminal {
			terminalCost += cost
		} else {
			dataCost += cost
		}
		return nil
	})
	for range 2 {
		if n, err := limited.Write([]byte{1}); n != 1 || err != nil {
			t.Fatal("finite complete frame refused", n, err)
		}
	}
	if n, err := limited.Write([]byte{1}); n != 0 || err == nil {
		t.Fatal("child exceeded complete-frame allowance", n, err)
	}
	if dataCost != 34 || limited.trafficUsed != 134 {
		t.Fatal("OPEN or headers were omitted", dataCost, limited.trafficUsed)
	}
	if err := limited.Close(); err != nil || terminalCost != 17 {
		t.Fatal("exhausted child lost prepaid CLOSE", err, terminalCost)
	}
	if n, err := ordinary.Write([]byte("ordinary")); n != 8 || err != nil || !s.Live() {
		t.Fatal("limited child retired ordinary sibling", n, err)
	}
	physical.mu.Lock()
	out := bytes.NewReader(bytes.Clone(physical.output))
	physical.mu.Unlock()
	for i, kind := range []uint8{ardp.KindBytes, ardp.KindBytes, ardp.KindClose, ardp.KindBytes} {
		frame, err := ardp.ReadFrame(out)
		if err != nil || frame.Kind != kind {
			t.Fatal("unexpected physical frame", i, frame.Kind, err)
		}
	}
	if out.Len() != 0 {
		t.Fatal("refused payload reached physical output")
	}
}

func TestSessionOutputAdmissionCountsCompleteFramesBeforeWire(t *testing.T) {
	for _, maximum := range []uint64{137, 84} {
		t.Run(map[uint64]string{137: "all-frames", 84: "refused-control"}[maximum], func(t *testing.T) {
			physical := newLifecycleConn(false)
			capacity := errors.New("session output owner exhausted")
			remaining := maximum
			var charges []uint64
			s := Prepare(t.Context(), physical, time.Now().Add(10*time.Second), 1<<20, nil, false, NewBudget(1<<20), Handlers{Output: func(cost uint64) error {
				charges = append(charges, cost)
				if cost > remaining {
					return capacity
				}
				remaining -= cost
				return nil
			}})
			t.Cleanup(func() { _ = s.Close() })
			body := ardp.EncodeOpen(ardp.Open{RecipientNodeID: [32]byte{1}, RecipientDutyGeneration: 1, Purpose: 7, Deadline: time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)}, false)
			lane, err := s.Open(t.Context(), t.Context(), body)
			if err != nil {
				t.Fatal(err)
			}
			// This output-only fixture creates an already active lane. BeginRole
			// belongs to incoming pending TLS children, covered separately above.
			if n, err := lane.Write([]byte{1, 2, 3}); n != 3 || err != nil {
				t.Fatal(n, err)
			}
			err = s.write(lane, ardp.Frame{Kind: ardp.KindCredit, Lane: lane.id, Body: []byte{0, 0, 0, 1}}, false)
			if maximum == 84 {
				if !errors.Is(err, capacity) || !errors.Is(lane.Close(), capacity) || !s.Live() || s.PhysicalFailure() != nil {
					t.Fatal("unstarted capacity refusal lost its provenance", err)
				}
				if len(charges) != 4 || charges[0] != 65 || charges[1] != 19 || charges[2] != 20 || charges[3] != 17 || s.used != 84 {
					t.Fatal("refused control escaped accounting", charges, s.used)
				}
			} else {
				if err != nil || lane.CloseWrite() != nil || lane.Close() != nil {
					t.Fatal("complete control frames refused", err)
				}
				if len(charges) != 5 || charges[0] != 65 || charges[1] != 19 || charges[2] != 20 || charges[3] != 16 || charges[4] != 17 || remaining != 0 {
					t.Fatal("complete frame headers or termination omitted", charges, remaining)
				}
			}
			physical.mu.Lock()
			wireBytes := len(physical.output)
			physical.mu.Unlock()
			if wireBytes != int(maximum) {
				t.Fatal("refused output reached the physical connection", wireBytes, maximum)
			}
			lane.Finish()
		})
	}
}

func TestOutputBudgetRefusesBeforeWireAndPreservesOrdinarySibling(t *testing.T) {
	for _, prepaid := range []bool{true, false} {
		t.Run(map[bool]string{true: "held-terminal", false: "refused-before-TLS"}[prepaid], func(t *testing.T) {
			capacity := errors.New("actual output owner exhausted")
			var terminalCalls int
			s, limited, ordinary, physical := constrainedIncomingChildren(t, 128<<10, func(cost uint64, terminal bool) error {
				if terminal {
					terminalCalls++
					if prepaid && cost == 17 && terminalCalls == 1 {
						return nil
					}
				}
				return capacity
			})
			if n, err := limited.Write([]byte("refuse")); n != 0 || !errors.Is(err, capacity) {
				t.Fatal("output was not refused by its real gate", n, err)
			}
			closeErr := limited.Close()
			if prepaid && closeErr != nil || !prepaid && !errors.Is(closeErr, capacity) || terminalCalls != 1 {
				t.Fatal("terminal ownership lost", closeErr, terminalCalls)
			}
			// Refusal cancels the incoming handler, which can send its prepaid
			// CLOSE before this caller reaches Close. Join that once-only output
			// before inspecting the wire; refused payload must never be present.
			physical.mu.Lock()
			wire := bytes.NewReader(bytes.Clone(physical.output))
			physical.mu.Unlock()
			if prepaid {
				frame, err := ardp.ReadFrame(wire)
				if err != nil || frame.Kind != ardp.KindClose || frame.Lane != limited.id || len(frame.Body) != 1 || frame.Body[0] > 1 {
					t.Fatal("refusal emitted something other than its prepaid CLOSE", frame, err)
				}
			}
			if wire.Len() != 0 || limited.trafficUsed != 100 {
				t.Fatal("refused output consumed wire or lane bytes", wire.Len(), limited.trafficUsed)
			}
			if !s.Live() || s.PhysicalFailure() != nil {
				t.Fatal("pre-output refusal became a Carrier failure")
			}
			if n, err := ordinary.Write([]byte("ordinary")); n != 8 || err != nil {
				t.Fatal("ordinary sibling lost physical progress", n, err)
			}
		})
	}
}

func TestChildTrafficDebitsControlHeadersBeforeOutput(t *testing.T) {
	var charges []uint64
	// OPEN=66, two terminal reserves=34, CREDIT=20, EOF=16.
	s, limited, ordinary, _ := constrainedIncomingChildren(t, 136, func(cost uint64, _ bool) error {
		charges = append(charges, cost)
		return nil
	})
	if err := s.write(limited, ardp.Frame{Kind: ardp.KindCredit, Lane: limited.id, Body: []byte{0, 0, 0, 1}}, false); err != nil {
		t.Fatal(err)
	}
	if err := limited.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if err := s.write(limited, ardp.Frame{Kind: ardp.KindCredit, Lane: limited.id, Body: []byte{0, 0, 0, 1}}, false); err == nil {
		t.Fatal("control output bypassed child allowance")
	}
	if err := limited.Close(); err != nil {
		t.Fatal("control exhaustion lost CLOSE", err)
	}
	if len(charges) != 3 || charges[0] != 20 || charges[1] != 16 || charges[2] != 17 {
		t.Fatal("complete control cost was not charged", charges)
	}
	if n, err := ordinary.Write([]byte{1}); n != 1 || err != nil || !s.Live() {
		t.Fatal("control refusal lost sibling progress", n, err)
	}
}

func TestChildTrafficInputLimitKeepsBufferedBytesAndOrdinarySibling(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	ready := make(chan *Lane, 2)
	s := Prepare(t.Context(), local, time.Now().Add(10*time.Second), 1<<20, nil, true, NewBudget(1<<20), Handlers{
		PrepareOpen: func(l *Lane, _ []byte) error {
			if l.id == 1 {
				// OPEN=66, terminals=34, one one-byte BYTES frame=17.
				return l.ConstrainTraffic(117, func(uint64, bool) error { return nil })
			}
			return nil
		},
		Open: func(ctx context.Context, l *Lane, _ []byte) error {
			ready <- l
			<-ctx.Done()
			return context.Cause(ctx)
		},
	})
	s.Start()
	t.Cleanup(func() { s.Retire(nil); _ = s.Close() })
	for _, id := range []uint32{1, 3} {
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: make([]byte, 50)}); err != nil {
			t.Fatal(err)
		}
	}
	limited, ordinary := <-ready, <-ready
	if limited.id != 1 {
		limited, ordinary = ordinary, limited
	}
	terminal := make(chan ardp.Frame, 1)
	go func() { f, _ := ardp.ReadFrame(remote); terminal <- f }()
	for _, frame := range []ardp.Frame{
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte{1}},
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte{2}},
		{Kind: ardp.KindBytes, Lane: 3, Body: []byte{3}},
	} {
		if err := ardp.WriteFrame(remote, frame); err != nil {
			t.Fatal(err)
		}
	}
	waitChildQueue(t, limited, 1, true)
	waitChildQueue(t, ordinary, 1, false)
	if f := <-terminal; f.Kind != ardp.KindClose || f.Lane != 1 {
		t.Fatal("limited input lost its terminal", f.Kind, f.Lane)
	}
	data := make([]byte, 1)
	if n, err := limited.Read(data); n != 1 || err != nil || data[0] != 1 {
		t.Fatal("already retained input was discarded", n, err, data)
	}
	if n, err := ordinary.Read(data); n != 1 || err != nil || data[0] != 3 || !s.Live() {
		t.Fatal("ordinary sibling lost input", n, err, data)
	}
}

func TestOutputGateCannotHideStartedPhysicalFailure(t *testing.T) {
	s, limited, _, physical := constrainedIncomingChildren(t, 128<<10, func(uint64, bool) error { return nil })
	failed := errors.New("late physical output failed")
	physical.mu.Lock()
	physical.partial = failed
	physical.mu.Unlock()
	if _, err := limited.Write([]byte("partial")); !errors.Is(err, failed) || localCapacityRefusal(err) {
		t.Fatal("physical failure became an unstarted budget refusal", err)
	}
	if s.Live() || !errors.Is(s.Close(), failed) || !errors.Is(s.PhysicalFailure(), failed) {
		t.Fatal("started physical failure was lost")
	}
}
