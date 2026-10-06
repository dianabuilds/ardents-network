package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These tests isolate physical queue ownership, not successful TLS, Network or
// Admission. The receiving owner supplies the actual child-group reservation.
func TestChildPreparationPrecedesPipelinedHandshakeInput(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	principal := NewBudget(1 << 20)
	group := NewBudget(128)
	preparing := make(chan *Lane, 1)
	continuePreparation := make(chan struct{})
	finishHandler := make(chan struct{})
	s := Prepare(t.Context(), local, time.Now().Add(10*time.Second), 1<<20, nil, true, principal, Handlers{
		PrepareOpen: func(l *Lane, body []byte) error {
			if err := l.ConstrainQueues(group); err != nil {
				return err
			}
			preparing <- l
			<-continuePreparation
			return nil
		},
		Open: func(ctx context.Context, l *Lane, body []byte) error { <-finishHandler; return context.Cause(ctx) },
	})
	s.Start()
	defer func() {
		s.Retire(nil)
		select {
		case <-continuePreparation:
		default:
			close(continuePreparation)
		}
		close(finishHandler)
		_ = s.Close()
	}()
	input := make(chan error, 1)
	go func() {
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: make([]byte, 50)}); err != nil {
			input <- err
			return
		}
		input <- ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("handshake")})
	}()
	l := <-preparing
	select {
	case err := <-input:
		t.Fatalf("pipelined bytes crossed unfinished preparation: %v", err)
	default:
	}
	group.mu.Lock()
	used := group.used
	group.mu.Unlock()
	if used != ardp.HeaderSize+1 {
		t.Fatal("terminal capacity was not retained before input", used)
	}
	close(continuePreparation)
	if err := <-input; err != nil {
		t.Fatal(err)
	}
	waitChildQueue(t, l, len("handshake"), false)
	group.mu.Lock()
	used = group.used
	group.mu.Unlock()
	if used != ardp.HeaderSize+1+uint64(len("handshake")) {
		t.Fatal("handshake bypassed shared queue", used)
	}
	data := make([]byte, len("handshake"))
	if n, err := l.Read(data); err != nil || n != len(data) || string(data) != "handshake" {
		t.Fatal("retained handshake unavailable", n, err)
	}
	group.mu.Lock()
	used = group.used
	group.mu.Unlock()
	if used != ardp.HeaderSize+1 {
		t.Fatal("consumption returned terminal capacity", used)
	}
	if err := l.ConstrainQueues(NewBudget(128)); err == nil {
		t.Fatal("child rebound to another queue")
	}
	// Parent retirement joins the receiving handler before queue return.
	s.Retire(nil)
}

func TestFullChildGroupStillClosesAndLeavesOrdinarySiblingUsable(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	group := NewBudget(ardp.HeaderSize + 1 + 3)
	lanes := make(chan *Lane, 2)
	finish := make(chan struct{})
	s := Prepare(t.Context(), local, time.Now().Add(10*time.Second), 1<<20, nil, true, NewBudget(1<<20), Handlers{
		PrepareOpen: func(l *Lane, body []byte) error {
			if l.id == 1 {
				return l.ConstrainQueues(group)
			}
			return nil
		},
		Open: func(ctx context.Context, l *Lane, body []byte) error { lanes <- l; <-finish; return context.Cause(ctx) },
	})
	s.Start()
	defer func() {
		s.Retire(nil)
		select {
		case <-finish:
		default:
			close(finish)
		}
		_ = s.Close()
	}()
	for _, id := range []uint32{1, 3} {
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: make([]byte, 50)}); err != nil {
			t.Fatal(err)
		}
	}
	first, sibling := <-lanes, <-lanes
	if first.id != 1 {
		first, sibling = sibling, first
	}
	for _, f := range []ardp.Frame{
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte("abc")},
		{Kind: ardp.KindBytes, Lane: 1, Body: []byte("x")},
		{Kind: ardp.KindBytes, Lane: 3, Body: []byte("ordinary")},
	} {
		if err := ardp.WriteFrame(remote, f); err != nil {
			t.Fatal(err)
		}
	}
	waitChildQueue(t, first, 3, true)
	waitChildQueue(t, sibling, len("ordinary"), false)
	if !s.Live() {
		t.Fatal("smaller child-group limit retired ordinary Carrier")
	}
	data := make([]byte, len("ordinary"))
	if n, err := sibling.Read(data); err != nil || n != len(data) || string(data) != "ordinary" {
		t.Fatal("ordinary sibling lost input", n, err)
	}
	closed := make(chan ardp.Frame, 1)
	go func() { f, _ := ardp.ReadFrame(remote); closed <- f }()
	if err := first.Close(); err != nil {
		t.Fatal("full group prevented terminal output", err)
	}
	if f := <-closed; f.Kind != ardp.KindClose || f.Lane != 1 {
		t.Fatal("reserved CLOSE missing", f.Kind, f.Lane)
	}
	first.Finish()
	group.mu.Lock()
	used := group.used
	group.mu.Unlock()
	if used != 3 {
		t.Fatal("physical finish released unread payload or retained termination", used)
	}
	s.Retire(nil)
	close(finish)
	_ = s.Close()
	group.mu.Lock()
	used = group.used
	group.mu.Unlock()
	if used != 0 {
		t.Fatal("joined parent retained child payload", used)
	}
}

func TestChildGroupAccountsQueuedOutputHeadersAndControls(t *testing.T) {
	physical := newLifecycleConn(true)
	var input bytes.Buffer
	for _, id := range []uint32{1, 3, 5} {
		if err := ardp.WriteFrame(&input, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: make([]byte, 50)}); err != nil {
			t.Fatal(err)
		}
	}
	physical.input = bytes.NewReader(input.Bytes())
	group := NewBudget(2*(ardp.HeaderSize+1) + ardp.HeaderSize + 3)
	lanes := make(chan *Lane, 3)
	finish := make(chan struct{})
	s := Prepare(t.Context(), physical, time.Now().Add(10*time.Second), 1<<20, nil, true, NewBudget(1<<20), Handlers{
		PrepareOpen: func(l *Lane, body []byte) error {
			if l.id != 5 {
				return l.ConstrainQueues(group)
			}
			return nil
		},
		Open: func(ctx context.Context, l *Lane, body []byte) error { lanes <- l; <-finish; return context.Cause(ctx) },
	})
	s.Start()
	defer func() {
		s.Retire(nil)
		select {
		case <-finish:
		default:
			close(finish)
		}
		_ = s.Close()
	}()
	children := make(map[uint32]*Lane)
	for range 3 {
		l := <-lanes
		children[l.id] = l
		if err := l.BeginRole(); err != nil {
			t.Fatal(err)
		}
	}
	written := make(chan error, 1)
	go func() { _, err := children[1].Write([]byte("abc")); written <- err }()
	<-physical.writes // The complete 19-byte frame is now held in physical output.
	if _, err := children[3].Write([]byte("x")); err == nil {
		t.Fatal("sibling output bypassed aggregate header charge")
	}
	if err := s.write(children[3], ardp.Frame{Kind: ardp.KindCredit, Lane: 3, Body: []byte{0, 0, 0, 1}}, false); err == nil {
		t.Fatal("CREDIT bypassed child-group queue")
	}
	ordinary := make(chan error, 1)
	go func() { _, err := children[5].Write([]byte("ordinary")); ordinary <- err }()
	close(physical.writeGate)
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if err := <-ordinary; err != nil {
		t.Fatal("ordinary sibling lost output", err)
	}
	s.Retire(nil)
	close(finish)
	if err := s.Close(); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	group.mu.Lock()
	used := group.used
	group.mu.Unlock()
	if used != 0 {
		t.Fatal("joined output retained group capacity", used)
	}
}

func waitChildQueue(t *testing.T, l *Lane, size int, closed bool) {
	t.Helper()
	for {
		l.s.mu.Lock()
		ready := len(l.buffer) == size && l.closed == closed
		changed := l.changed
		l.s.mu.Unlock()
		if ready {
			return
		}
		select {
		case <-changed:
		case <-time.After(5 * time.Second):
			t.Fatal("child queue transition did not complete")
		}
	}
}
