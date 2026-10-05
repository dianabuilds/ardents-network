//go:build linux

package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Mechanical failure controls seed one already-received byte under its owner.
// They mint no Network authority, token, Grant, successful ACK or spend result.
func consumedCreditLane(t *testing.T, s *session, id uint32) *lane {
	t.Helper()
	l := lifecycleLane(t, s, id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.queues.reserve(1) {
		t.Fatal("input capacity absent")
	}
	l.buffer, l.receive = []byte{42}, window-1
	s.queued++
	return l
}

func TestCREDITQueuedRetirementNeverReachesPhysicalOutput(t *testing.T) {
	physical := newLifecycleConn(true)
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	active := lifecycleLane(t, s, 1)
	credit := consumedCreditLane(t, s, 3)
	written := make(chan error, 1)
	go func() { _, err := active.Write([]byte("active")); written <- err }()
	<-physical.writes
	consumed := make(chan error, 1)
	go func() {
		var byteRead [1]byte
		n, err := credit.Read(byteRead[:])
		if n != 1 || byteRead[0] != 42 {
			err = errors.Join(err, errors.New("consumption differs"))
		}
		consumed <- err
	}()
	until := time.Now().Add(2 * time.Second)
	queued := false
	for time.Now().Before(until) {
		s.mu.Lock()
		for _, turn := range s.output {
			if turn.lane == credit {
				queued = true
			}
		}
		s.mu.Unlock()
		if queued {
			break
		}
		runtime.Gosched()
	}
	if !queued {
		t.Fatal("CREDIT did not queue behind actual writer")
	}
	if err := credit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, consumed); !errors.Is(err, net.ErrClosed) {
		t.Fatal("wrong queued refusal", err)
	}
	select {
	case raw := <-physical.writes:
		t.Fatalf("queued CREDIT emitted: %x", raw)
	default:
	}
	s.mu.Lock()
	failed, charged := s.stopped, s.used
	s.mu.Unlock()
	if failed || charged != uint64(ardp.HeaderSize+len("active")) {
		t.Fatal("unemitted CREDIT poisoned or charged parent", failed, charged)
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); err != nil {
		t.Fatal(err)
	}
	if s.joinedPhysicalFailure() != nil {
		t.Fatal("queued refusal recorded as started physical failure")
	}
}

func TestCREDITStartedFailureHeldUntilJoined(t *testing.T) {
	late := errors.New("exact started CREDIT failure")
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose, physical.partial = true, late
	queues := &queueBudget{maximum: 64 << 20}
	release, err := queues.channel()
	if err != nil {
		t.Fatal(err)
	}
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, queues, nil)
	l := consumedCreditLane(t, s, 1)
	read := make(chan error, 1)
	go func() { var value [1]byte; _, err := l.Read(value[:]); read <- err }()
	raw := <-physical.writes
	frame, err := ardp.ReadFrame(bytes.NewReader(raw))
	if err != nil || frame.Kind != ardp.KindCredit || frame.Lane != 1 || !bytes.Equal(frame.Body, []byte{0, 0, 0, 1}) {
		t.Fatal("wrong started physical frame", frame, err)
	}
	joined := make(chan error, 1)
	go func() { err := s.Close(); release(); joined <- err }()
	<-physical.closed
	select {
	case err := <-joined:
		t.Fatal("started writer not joined", err)
	default:
	}
	queues.mu.Lock()
	held := queues.used
	queues.mu.Unlock()
	if held != 16<<10 {
		t.Fatal("original capacity released before writer joined", held)
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, read); !errors.Is(err, late) {
		t.Fatal("CREDIT cause lost", err)
	}
	first := lifecycleResult(t, joined)
	if !errors.Is(first, late) || s.Close() != first {
		t.Fatal("joined result lost/replaced", first)
	}
	physicalFailure := s.joinedPhysicalFailure()
	if TerminalFailureStage(physicalFailure) != "physical-write" {
		t.Fatal("wrong failure phase", physicalFailure)
	}
	var started *physicalWriteFailure
	if !errors.As(physicalFailure, &started) || started.kind != ardp.KindCredit || started.cause != late {
		t.Fatal("physical provenance differs", physicalFailure)
	}
	s.mu.Lock()
	charged := s.used
	s.mu.Unlock()
	if charged != ardp.HeaderSize+4 {
		t.Fatal("failed started CREDIT refunded original byte charge", charged)
	}
	queues.mu.Lock()
	held = queues.used
	queues.mu.Unlock()
	if held != 0 {
		t.Fatal("joined capacity leaked", held)
	}
}

func TestTerminalFailureStageRejectsEveryUnrelatedLeaf(t *testing.T) {
	peer := &physicalWriteFailure{kind: ardp.KindCredit, cause: syscall.EPIPE}
	if TerminalFailureStage(fmt.Errorf("receiving framing: %w", peer)) != "peer-retired-write" {
		t.Fatal("exact physical cause not classified")
	}
	foreign := errors.New("unrelated durable release failure")
	for _, err := range []error{errors.Join(peer, foreign), fmt.Errorf("receiving: %w", errors.Join(foreign, peer)), errors.Join(peer, fmt.Errorf("wrapped unrelated: %w", foreign))} {
		if TerminalFailureStage(err) != "terminal" {
			t.Fatal("unrelated aggregate leaf hidden", err)
		}
		if !errors.Is(err, foreign) {
			t.Fatal("classification discarded cause")
		}
	}
	if TerminalFailureStage(&physicalWriteFailure{kind: ardp.KindCredit, cause: errors.Join(syscall.EPIPE, foreign)}) != "physical-write" {
		t.Fatal("joined native cause treated as known peer retirement")
	}
	if TerminalFailureStage(syscall.EPIPE) != "terminal" {
		t.Fatal("unproved phase minted from a native error")
	}
}

func TestTerminalFailureStageRetainsPeerWriteAndOwnedClose(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.partial, physical.closeFailure = syscall.EPIPE, syscall.EPIPE
	s := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	l := lifecycleLane(t, s, 1)
	l.openEmitted = true
	if err := l.Close(); !errors.Is(err, syscall.EPIPE) {
		t.Fatal("actual peer write failure was lost", err)
	}
	first := s.Close()
	if !errors.Is(first, syscall.EPIPE) || TerminalFailureStage(s.joinedPhysicalFailure()) != "peer-retired-write" {
		t.Fatal("owned peer close vetoed exact physical write provenance", first)
	}
	if s.Close() != first {
		t.Fatal("classification changed retained terminal result")
	}
}

func TestTerminalFailureStageCloseProvenanceCannotBorrowAnotherWrite(t *testing.T) {
	failed := func(closeCause error, write bool) *session {
		physical := newLifecycleConn(false)
		physical.closeFailure = closeCause
		s := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
		if write {
			physical.partial = syscall.EPIPE
			l := lifecycleLane(t, s, 1)
			l.openEmitted = true
			if err := l.Close(); !errors.Is(err, syscall.EPIPE) {
				t.Fatal(err)
			}
		}
		first := s.Close()
		if first == nil || s.Close() != first {
			t.Fatal("physical result was discarded or replaced", first)
		}
		return s
	}
	closeOnly := failed(syscall.EPIPE, false)
	if TerminalFailureStage(closeOnly.joinedPhysicalFailure()) != "terminal" {
		t.Fatal("close-only failure invented actual write provenance")
	}
	foreign := errors.New("unrelated physical close failed")
	mixed := failed(foreign, true)
	if err := mixed.joinedPhysicalFailure(); TerminalFailureStage(err) != "terminal" || !errors.Is(err, foreign) {
		t.Fatal("foreign close hidden behind peer write", err)
	}
	other := failed(nil, true)
	if err := errors.Join(other.joinedPhysicalFailure(), closeOnly.joinedPhysicalFailure()); TerminalFailureStage(err) != "terminal" {
		t.Fatal("another owner supplied close's write witness", err)
	}
	valid := failed(syscall.EPIPE, true)
	nested := fmt.Errorf("receiving: %w", errors.Join(valid.joinedPhysicalFailure(), fmt.Errorf("release: %w", foreign)))
	if TerminalFailureStage(nested) != "terminal" || !errors.Is(nested, foreign) {
		t.Fatal("nested foreign leaf was discarded", nested)
	}
}

func TestNestedCloseRetainsActualLowerWriteOwner(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.partial = syscall.EPIPE
	parent := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer parent.Close()
	lower := lifecycleLane(t, parent, 1)
	lower.openEmitted = true
	child := newSession(t.Context(), &retiredConn{Conn: lower}, lower.end, 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	first := child.Close()
	if !errors.Is(first, syscall.EPIPE) || child.Close() != first {
		t.Fatal("nested Close lost or replaced actual lower write failure", first)
	}
	var write *physicalWriteFailure
	if !errors.As(child.joinedPhysicalFailure(), &write) || write.owner != parent {
		t.Fatal("nested Close did not retain actual lower write owner", child.joinedPhysicalFailure())
	}
	parent.Close()
	var retained *physicalWriteFailure
	if !errors.As(parent.joinedPhysicalFailure(), &retained) || retained != write {
		t.Fatal("nested Close replaced the lower owner's actual witness")
	}
	var upperClose *physicalCloseFailure
	if errors.As(child.joinedPhysicalFailure(), &upperClose) {
		t.Fatal("borrower Close minted a physical socket-close witness", upperClose)
	}
	if TerminalFailureStage(child.joinedPhysicalFailure()) != "peer-retired-write" {
		t.Fatal("nested borrower close invented another physical owner", child.joinedPhysicalFailure())
	}
	if len(physical.output) != 3 {
		t.Fatal("control did not enter actual partial physical output", len(physical.output))
	}
}
