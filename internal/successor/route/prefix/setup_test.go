package prefix

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

func TestPublicPrefixOpeningRefusesCanceledCallerAndSealedOriginalBeforeEffects(t *testing.T) {
	var observations, presentations, returns int
	end := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	config := Config{Deadline: end,
		Current: func() (network.RuntimeView, error) {
			observations++
			return network.RuntimeView{}, errors.New("no accepting authority fixture")
		},
		Present: func(context.Context, ardp.Hello) ([]byte, error) {
			presentations++
			return nil, errors.New("no successful presentation fixture")
		},
		Release: func() error { returns++; return nil },
	}
	config.Leg.NotAfter = end
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := Open(caller, config); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original caller entered Prefix opening", err)
	}
	source, _, _, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	source.config.Leg.EntryMember.RoleDomain = 1
	config.Leg.EntryMember.RoleDomain = 3
	source.Seal()
	if got, err := OpenResponder(t.Context(), source, config); got != nil || err == nil {
		t.Fatal("sealed original Source produced a Responder")
	}
	if observations != 0 || presentations != 0 || returns != 0 {
		t.Fatal("pre-handoff refusal reached authority/presentation or returned caller-owned reservation", observations, presentations, returns)
	}
}

func TestResponderRecipientRefusesForeignSourceBeforeObservation(t *testing.T) {
	source, _, _, closeSource := terminalSetupPhysicalPrefix(t)
	defer closeSource()
	source.config.Leg.EntryMember.RoleDomain = 1
	foreign := &Prefix{}
	responder, _, _, closeResponder := terminalSetupPhysicalPrefix(t)
	defer closeResponder()
	responder.source = foreign
	responder.config.Leg.EntryMember.RoleDomain = 3
	observations := 0
	observe := func() (network.RuntimeView, error) {
		observations++
		return network.RuntimeView{}, errors.New("no successful authority fixture")
	}
	source.config.Current, responder.config.Current = observe, observe
	if err := source.CheckResponderRendezvous(responder, network.RetainedDuty{}, nil); err == nil {
		t.Fatal("foreign Source supplied recipient authority")
	}
	if observations != 0 {
		t.Fatal("foreign Source reached Network observation", observations)
	}
}

// This wrapper injects only cleanup failures around a real physical pipe.
// Every deadline and Close operation still reaches that original connection.
type prefixSetupFaultConn struct {
	net.Conn
	deadlineFailure, closeFailure error
	closeStarted, closeGate       chan struct{}
	deadlines, closes             atomic.Uint32
}

func (c *prefixSetupFaultConn) SetDeadline(end time.Time) error {
	c.deadlines.Add(1)
	return errors.Join(c.Conn.SetDeadline(end), c.deadlineFailure)
}
func (c *prefixSetupFaultConn) Close() error {
	c.closes.Add(1)
	if c.closeStarted != nil {
		close(c.closeStarted)
	}
	if c.closeGate != nil {
		<-c.closeGate
	}
	return errors.Join(c.Conn.Close(), c.closeFailure)
}

func TestPrefixSetupInterruptRetainsDeadlineFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer peer.Close()
		defer local.Close()
		sentinel := errors.New("original setup deadline operation failed")
		physical := &prefixSetupFaultConn{Conn: local, deadlineFailure: sentinel, closeStarted: make(chan struct{})}
		ctx, cancel := context.WithCancel(t.Context())
		stop := interruptPrefixOpening(ctx, physical)
		cancel()
		<-physical.closeStarted
		result := stop()
		if !errors.Is(result, sentinel) {
			t.Fatal("deadline-only interruption cause lost", result)
		}
		if stop() != result {
			t.Fatal("repeated setup join replaced terminal error")
		}
		if physical.deadlines.Load() != 1 || physical.closes.Load() != 1 {
			t.Fatal("original setup operations repeated", physical.deadlines.Load(), physical.closes.Load())
		}
		if _, err := peer.Write([]byte{1}); err == nil {
			t.Fatal("deadline fault replaced actual physical close")
		}
	})
}

func TestPrefixSetupInterruptJoinsLateCloseBeforePublishingResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer peer.Close()
		defer local.Close()
		deadline := errors.New("setup deadline failure")
		late := errors.New("late original physical close failure")
		gate := make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(gate) }) }
		defer unblock()
		physical := &prefixSetupFaultConn{Conn: local, deadlineFailure: deadline, closeFailure: late, closeStarted: make(chan struct{}), closeGate: gate}
		ctx, cancel := context.WithCancel(t.Context())
		stop := interruptPrefixOpening(ctx, physical)
		cancel()
		<-physical.closeStarted
		result := make(chan error, 1)
		go func() { result <- stop() }()
		synctest.Wait()
		select {
		case err := <-result:
			t.Fatal("setup cleanup returned before actual Close joined", err)
		default:
		}
		unblock()
		joined := <-result
		if !errors.Is(joined, deadline) || !errors.Is(joined, late) {
			t.Fatal("joined setup result lost original physical failures", joined)
		}
		if stop() != joined || physical.closes.Load() != 1 {
			t.Fatal("setup repeated close lost original terminal result")
		}
	})
}

func TestPrefixSetupStoppedBeforeCancellationDoesNotInterruptPhysicalConn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer peer.Close()
		defer local.Close()
		physical := &prefixSetupFaultConn{Conn: local}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		stop := interruptPrefixOpening(ctx, physical)
		if err := stop(); err != nil {
			t.Fatal(err)
		}
		cancel()
		synctest.Wait()
		if err := stop(); err != nil {
			t.Fatal("stopped setup callback revived", err)
		}
		if physical.deadlines.Load() != 0 || physical.closes.Load() != 0 {
			t.Fatal("post-publication caller cancellation reached stopped setup callback")
		}
		done := make(chan error, 1)
		go func() { _, err := physical.Write([]byte{42}); done <- err }()
		var body [1]byte
		if n, err := peer.Read(body[:]); err != nil || n != 1 || body[0] != 42 {
			t.Fatal("stopped setup callback poisoned original connection", n, body, err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

// These setup/publication oracles use real pipe parents and observe physical
// return. They supply no role authority, successful Registration or Admission.
func TestPrefixTerminalSetupSealRefusesHandoffAndWaitsForCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, readerDone, returns, cleanup := terminalSetupPhysicalPrefix(t)
		defer cleanup()
		child, cancel, opening, err := p.beginTerminalOpening(t.Context(), time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		defer opening.finish()
		p.Seal()
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatal("seal left original setup live", child.Err())
		}
		borrow := &Borrow{owner: p, interrupt: func() {}, join: func() error { return nil }, active: func() bool { return false }}
		if err := opening.publishBorrow(t.Context(), child, borrow); !errors.Is(err, net.ErrClosed) {
			t.Fatal("sealed original setup published borrower", err)
		}
		joined := make(chan error, 1)
		go func() { joined <- p.Close() }()
		synctest.Wait()
		select {
		case <-readerDone:
			t.Fatal("parent interrupted before setup completed")
		default:
		}
		if returns.Load() != 0 {
			t.Fatal("capacity returned before setup completion")
		}
		opening.finish()
		opening.finish() // Repeated completion cannot decrement another claim.
		select {
		case err := <-joined:
			if err != nil || returns.Load() != 1 {
				t.Fatal("joined setup return differs", err, returns.Load())
			}
		case <-time.After(time.Second):
			t.Fatal("prefix did not join original setup")
		}
	})
}

func TestPrefixTerminalSetupTransfersOnlyOneExactBorrow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, readerDone, returns, cleanup := terminalSetupPhysicalPrefix(t)
		defer cleanup()
		other, _, _, otherCleanup := terminalSetupPhysicalPrefix(t)
		defer otherCleanup()
		child, cancel, opening, err := p.beginTerminalOpening(t.Context(), time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		defer opening.finish()
		interrupted, joined := make(chan struct{}), make(chan struct{})
		var interruptOnce, joinedOnce sync.Once
		unblock := func() { joinedOnce.Do(func() { close(joined) }) }
		defer unblock()
		borrow := &Borrow{owner: other, interrupt: func() { interruptOnce.Do(func() { close(interrupted) }) }, active: func() bool { return false }}
		borrow.join = func() error { <-joined; borrow.ReturnJoined(); return nil }
		if err := opening.publishBorrow(t.Context(), child, borrow); err == nil {
			t.Fatal("setup transferred to another generation")
		}
		borrow.owner = p
		if err := opening.publishBorrow(t.Context(), child, borrow); err != nil {
			t.Fatal(err)
		}
		if err := opening.publishBorrow(t.Context(), child, borrow); err == nil {
			t.Fatal("setup published twice")
		}
		opening.finish()
		if err := opening.publishBorrow(t.Context(), child, borrow); err == nil {
			t.Fatal("completed setup published again")
		}
		closed := make(chan error, 1)
		go func() { closed <- p.Close() }()
		select {
		case <-interrupted:
		case <-time.After(time.Second):
			t.Fatal("published original borrower not interrupted")
		}
		synctest.Wait()
		select {
		case <-readerDone:
			t.Fatal("parents retired before transferred borrower joined")
		default:
		}
		if returns.Load() != 0 {
			t.Fatal("setup finish returned published physical capacity")
		}
		unblock()
		select {
		case err := <-closed:
			if err != nil || returns.Load() != 1 {
				t.Fatal("physical borrow return differs", err, returns.Load())
			}
		case <-time.After(time.Second):
			t.Fatal("published borrower not joined")
		}
	})
}

// Local retirement must not let the readiness watcher cancel a still-live
// sibling between Close's Interior-before-Entry retirement stages.
func TestPrefixSetupLocalParentRetirementPreservesSiblingUntilClose(t *testing.T) {
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	p.Seal()
	p.interior.Retire(nil)
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("watcher did not observe original Interior retirement")
	}
	if err := p.ctx.Err(); err != nil {
		t.Fatal("local Interior retirement canceled original Entry before its close", err)
	}
	if !p.entry.Live() || returns.Load() != 0 {
		t.Fatal("local retirement closed or returned the original sibling early")
	}
	if err := p.Close(); err != nil || returns.Load() != 1 {
		t.Fatal("original parent retirement did not join cleanly once", err, returns.Load())
	}
}

func TestPrefixSetupUnexpectedParentRetirementStillCancelsSibling(t *testing.T) {
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	failure := errors.New("original Interior failed before local retirement")
	p.interior.Retire(failure)
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("watcher did not observe failed original parent")
	}
	if p.ctx.Err() == nil || returns.Load() != 0 {
		t.Fatal("unexpected original parent failure did not revoke the sibling before join")
	}
	if err := p.Close(); !errors.Is(err, failure) || returns.Load() != 1 {
		t.Fatal("parent join lost original failure or returned capacity incorrectly", err, returns.Load())
	}
}

// Explicit Close joins physical readers and returns capacity. Done is a
// readiness notification and is deliberately not this fixture's join oracle.
func terminalSetupPhysicalPrefix(t *testing.T, original ...context.Context) (*Prefix, <-chan struct{}, *atomic.Int32, func()) {
	t.Helper()
	caller := t.Context()
	if len(original) != 0 {
		caller = original[0]
	}
	ctx, cancel := context.WithCancel(caller)
	end := time.Now().Add(time.Minute)
	entry, entryPeer := net.Pipe()
	interior, interiorPeer := net.Pipe()
	queues := framing.NewBudget(4 << 20)
	p := &Prefix{ctx: ctx, caller: caller, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), activity: make(chan struct{}, 1)}
	p.entry = framing.New(ctx, entry, end, 32<<20, nil, false, queues, nil)
	p.interior = framing.New(ctx, interior, end, 32<<20, nil, false, queues, nil)
	returns := new(atomic.Int32)
	p.release = func() error {
		for _, reader := range []<-chan struct{}{p.entry.Done(), p.interior.Done()} {
			select {
			case <-reader:
			default:
				t.Error("capacity returned before original reader joined")
			}
		}
		returns.Add(1)
		return nil
	}
	go p.watch(func() error { return nil }, make(chan time.Time))
	cleanup := func() { _ = p.Close(); _ = entryPeer.Close(); _ = interiorPeer.Close(); cancel() }
	return p, p.entry.Done(), returns, cleanup
}

// This caller has a real canceled Context. Only propagation of cancellation to
// its derived physical context is deferred; no Network or admission succeeds.
type prefixDeferredPropagation struct{ context.Context }

func (c *prefixDeferredPropagation) Value(any) any { return nil }
func (c *prefixDeferredPropagation) AfterFunc(func()) func() bool {
	return func() bool { return true }
}

// Only an actual failed-opening provenance may poison the composing Context.
// General operation errors cannot manufacture that physical terminal result.
func TestOpeningRetirementPreservesOnlyPhysicalProvenance(t *testing.T) {
	operation := errors.New("opening refused before transfer")
	if err := OpeningRetirement(errors.Join(operation, context.Canceled, io.EOF)); err != nil {
		t.Fatal("operation error fabricated retirement", err)
	}
	cause := errors.New("original physical return failed")
	retained := OpeningRetirement(errors.Join(context.Canceled, &prefixOpeningFailure{operation: operation, retirement: errors.Join(io.ErrUnexpectedEOF, cause)}))
	if !errors.Is(retained, cause) || !errors.Is(retained, io.ErrUnexpectedEOF) || errors.Is(retained, operation) {
		t.Fatal("physical provenance discarded or replaced", retained)
	}
}
