package channel

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// This framing probe supplies bytes, not a genuine Admission Grant. Genuine
// authority, stock and spend still require separate command integration.
func TestHolderRefillAccountsAdmitAndAccept(t *testing.T) {
	for _, remaining := range []uint64{372, 400} {
		t.Run(strconv.FormatUint(remaining, 10), func(t *testing.T) { testHolderRefillAccounting(t, remaining) })
	}
}

func testHolderRefillAccounting(t *testing.T, remainingBeforeAdmit uint64) {
	t.Helper()
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().Add(2 * time.Second)
	s := New(t.Context(), local, end, remainingBeforeAdmit, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	seen := make(chan error, 1)
	go func() {
		frame, err := ardp.ReadFrame(peer)
		if err == nil && (frame.Kind != ardp.KindAdmit || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != 2) {
			err = errors.New("wrong holder ADMIT bytes")
		}
		if err == nil {
			accept, e := ardp.AcceptFrame(0, Window)
			err = e
			if err == nil {
				err = ardp.WriteFrame(peer, accept)
			}
		}
		seen <- err
	}()
	if err := probeParentExchange(s, t.Context(), t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil }); err != nil {
		t.Fatal(err)
	}
	if err := <-seen; err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	used, remaining := s.used, s.limit-s.used
	s.mu.Unlock()
	if used != 392 || remaining != 33554432-21 {
		t.Fatalf("ADMIT/ACCEPT debit or replacement wrong: used=%d remaining=%d", used, remaining)
	}
}

func TestHolderLostAcceptCancellationRetiresParent(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().Add(2 * time.Second)
	s := New(t.Context(), local, end, 400, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	read := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); read <- err; cancel() }()
	err := probeParentExchange(s, ctx, t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil })
	if e := <-read; e != nil {
		t.Fatal(e)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("lost ACK succeeded", err)
	}
	s.mu.Lock()
	stopped, used := s.stopped, s.used
	s.mu.Unlock()
	if !stopped || used != 371 {
		t.Fatalf("uncertain parent reusable or token uncharged: stopped=%v used=%d", stopped, used)
	}
}

func TestHolderCapacityRefusalStartsNoAdmitAndKeepsParent(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().Add(2 * time.Second)
	s := New(t.Context(), local, end, 400, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	observed := make(chan ardp.Frame, 1)
	go func() {
		frame, err := ardp.ReadFrame(peer)
		if err == nil {
			observed <- frame
		}
	}()
	capacityFailure := errors.New("actual local capacity refusal probe")
	called := false
	err := probeParentExchange(s, t.Context(), t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil }, func(_ context.Context, delta uint64) error {
		called = true
		if delta != 33554432-29 {
			t.Error("wrong additional exposure", delta)
		}
		s.mu.Lock()
		writer, used := len(s.writer), s.used
		s.mu.Unlock()
		if writer != 0 || used != 0 {
			t.Error("Hosting called inside writer or after ADMIT debit", writer, used)
		}
		return capacityFailure
	})
	if !called || !errors.Is(err, capacityFailure) {
		t.Fatal("capacity refusal not retained", err)
	}
	s.mu.Lock()
	stopped, used := s.stopped, s.used
	s.mu.Unlock()
	if stopped || used != 0 {
		t.Fatalf("wholly unemitted refusal retired sibling/chargedbytes: %v/%d", stopped, used)
	}
	select {
	case frame := <-observed:
		t.Fatal("capacity refusal emitted ADMIT", frame.Kind)
	default:
	}
}

func TestHolderRefillReservationAllowsChildControlAndTopsUpDelta(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().Add(2 * time.Second)
	s := New(t.Context(), local, end, 450, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	peerResult := make(chan error, 1)
	go func() {
		first, err := ardp.ReadFrame(peer)
		if err == nil && (first.Kind != ardp.KindCredit || first.Lane != 1) {
			err = errors.New("child control did not precede ADMIT")
		}
		if err == nil {
			second, e := ardp.ReadFrame(peer)
			err = e
			if err == nil && (second.Kind != ardp.KindAdmit || second.Lane != 0) {
				err = errors.New("ADMIT absent after capacity topup")
			}
		}
		if err == nil {
			ack, e := ardp.AcceptFrame(0, Window)
			err = e
			if err == nil {
				err = ardp.WriteFrame(peer, ack)
			}
		}
		peerResult <- err
	}()
	calls := 0
	var held uint64
	err := probeParentExchange(s, t.Context(), t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil }, func(_ context.Context, delta uint64) error {
		calls++
		held += delta
		if calls == 1 {
			child := &Lane{s: s, id: 1, end: end, writeEnd: end, ctx: t.Context(), changed: make(chan struct{})}
			return s.write(child, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}, false)
		}
		if calls != 2 || delta != 20 {
			t.Error("wrong missing capacity", calls, delta)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = <-peerResult; err != nil {
		t.Fatal(err)
	}
	if calls != 2 || held != 33554432-59 {
		t.Fatalf("double/omitted reserve: calls=%d held=%d", calls, held)
	}
	s.mu.Lock()
	used, remaining := s.used, s.limit-s.used
	s.mu.Unlock()
	if used != 412 || remaining != 33554432-21 {
		t.Fatalf("child/refill accounting: used=%d remaining=%d", used, remaining)
	}
}

func TestHolderZeroOriginalReserveCannotRequestCapacity(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().Add(time.Second)
	s := New(t.Context(), local, end, 371, nil, false, NewBudget(64<<20), nil)
	defer s.Close()
	called := false
	err := probeParentExchange(s, t.Context(), t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil }, func(context.Context, uint64) error { called = true; return errors.New("unexpected capacity request") })
	if err == nil || called {
		t.Fatal("zero remainder reached effects", err, called)
	}
}

// Delayed derived cancellation must not permit output after the original
// operation has already refused. This is a physical framing/cancellation probe,
// not a successful authority or Admission substitute.
func TestHolderRefillCanceledDuringCurrentnessStartsNoAdmit(t *testing.T) {
	conn := newLifecycleConn(false)
	operation := &refillDeferredCaller{Context: context.Background()}
	var s *Session
	selected := false
	check := func() error {
		if s == nil {
			return nil
		}
		s.mu.Lock()
		writerSelected := len(s.writer) != 0
		s.mu.Unlock()
		if writerSelected {
			// The writer has selected its turn and its observation completes
			// after cancellation; propagation into lane state remains delayed.
			selected = true
			operation.canceled.Store(true)
		}
		return nil
	}
	end := time.Now().Add(time.Second)
	s = New(t.Context(), conn, end, 400, check, false, NewBudget(64<<20), nil)
	defer s.Close()
	err := probeParentExchange(s, operation, t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil })
	if err == nil {
		t.Fatal("canceled operation succeeded")
	}
	s.mu.Lock()
	used := s.used
	s.mu.Unlock()
	conn.mu.Lock()
	emitted := len(conn.output)
	conn.mu.Unlock()
	if !selected || used != 0 || emitted != 0 {
		t.Fatalf("canceled operation started ADMIT: selected=%v used=%d emitted=%d", selected, used, emitted)
	}
}

func TestHolderInvalidRefillAcceptCannotReplaceAllowance(t *testing.T) {
	for _, test := range []struct {
		name   string
		status byte
		credit uint32
	}{{"refusal", 1, 0}, {"wrong-credit", 0, 1}} {
		t.Run(test.name, func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			end := time.Now().Add(time.Second)
			s := New(t.Context(), local, end, 400, nil, false, NewBudget(64<<20), nil)
			defer s.Close()
			peerResult := make(chan error, 1)
			go func() {
				frame, err := ardp.ReadFrame(peer)
				if err == nil && (frame.Kind != ardp.KindAdmit || frame.Lane != 0) {
					err = errors.New("expected original parent ADMIT")
				}
				if err == nil {
					ack, e := ardp.AcceptFrame(test.status, test.credit)
					err = e
					if err == nil {
						err = ardp.WriteFrame(peer, ack)
					}
				}
				peerResult <- err
			}()
			err := probeParentExchange(s, t.Context(), t.Context(), ardp.Hello{Purpose: ardp.PurposeForwarding, Deadline: end}, func(context.Context, ardp.Hello) ([]byte, error) { return make([]byte, 354), nil })
			if e := <-peerResult; e != nil {
				t.Fatal(e)
			}
			if err == nil {
				t.Fatal("invalid acknowledgement succeeded")
			}
			s.mu.Lock()
			stopped, limit, used := s.stopped, s.limit, s.used
			s.mu.Unlock()
			if !stopped || limit != 400 || used != 371 {
				t.Fatalf("invalid ACK restored/reused parent: stopped=%v limit=%d used=%d", stopped, limit, used)
			}
		})
	}
}

// probeParentExchange supplies a fixed wire probe and an independent replacement
// allowance. It exercises the channel, not holder presentation or token policy.
func probeParentExchange(s *Session, ctx, caller context.Context, hello ardp.Hello, present func(context.Context, ardp.Hello) ([]byte, error), holds ...func(context.Context, uint64) error) error {
	return s.ExchangeParent(ctx, caller, hello.Deadline, 33554432, func(ctx context.Context) (ardp.Frame, error) {
		raw, err := present(ctx, hello)
		return ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, raw...)}, err
	}, holds...)
}

type refillDeferredCaller struct {
	context.Context
	canceled atomic.Bool
}

func (c *refillDeferredCaller) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}
