//go:build linux

package route

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func sourceChannelsFixture(t *testing.T) (*closedSourceChannels, net.Conn, time.Time) {
	t.Helper()
	local, remote := net.Pipe()
	end := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	owner := newClosedSourceChannelOwner(local, end, local.Close)
	owner.start()
	t.Cleanup(func() { _ = owner.Close(); _ = remote.Close() })
	return owner, remote, end
}

func sourceIssuerOpen(end time.Time) ClosedOpen {
	return ClosedOpen{NextNodeID: [32]byte{1}, NextDutyGeneration: 1, Purpose: ardp.PurposeIssuer, Deadline: end}
}

func waitSourceChannelState(t *testing.T, owner *closedSourceChannels, condition func() bool) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for {
		owner.mu.Lock()
		ok, changed := condition(), owner.changed
		owner.mu.Unlock()
		if ok {
			return
		}
		if err := waitClosedSourceChange(changed, until); err != nil {
			t.Fatal("source channel state was not reached")
		}
	}
}

func TestClosedSourceChannelsConcurrentOpenPreservesWireOrder(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	const count = 64
	received := make(chan error, 1)
	go func() {
		for index := uint32(0); index < count; index++ {
			frame, err := ardp.ReadFrame(peer)
			if err != nil {
				received <- err
				return
			}
			if frame.Kind != ardp.KindOpen || frame.Lane != index*2+1 {
				received <- errors.New("concurrent OPEN IDs reached wire out of order")
				return
			}
		}
		received <- nil
	}()
	var workers sync.WaitGroup
	errs := make(chan error, count)
	for range count {
		workers.Go(func() {
			_, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(5*time.Second))
			errs <- err
		})
	}
	workers.Wait()
	for range count {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
}

func TestClosedSourceChannelsBoundControlPriorityBeforeQueuedData(t *testing.T) {
	owner := &closedSourceChannels{}
	controlOne := &closedSourceWrite{}
	controlTwo := &closedSourceWrite{}
	dataOne := &closedSourceWrite{}
	dataTwo := &closedSourceWrite{}
	owner.controls = []*closedSourceWrite{controlOne, controlTwo}
	owner.data = []*closedSourceWrite{dataOne, dataTwo}

	for index, want := range []struct {
		request *closedSourceWrite
		control bool
	}{{controlOne, true}, {dataOne, false}, {controlTwo, true}, {dataTwo, false}} {
		got, control := owner.nextWriteLocked()
		if got != want.request || control != want.control {
			t.Fatalf("schedule %d = %p/%t, want %p/%t", index, got, control, want.request, want.control)
		}
	}
}

func TestClosedSourceChannelsTerminalPriorityYieldsToQueuedData(t *testing.T) {
	owner := &closedSourceChannels{dataDue: true}
	terminalOne := &closedSourceWrite{control: true, terminal: true}
	terminalTwo := &closedSourceWrite{control: true, terminal: true}
	dataOne := &closedSourceWrite{}
	dataTwo := &closedSourceWrite{}
	owner.terminals = []*closedSourceWrite{terminalOne, terminalTwo}
	owner.data = []*closedSourceWrite{dataOne, dataTwo}

	for index, want := range []struct {
		request *closedSourceWrite
		control bool
	}{{terminalOne, true}, {dataOne, false}, {terminalTwo, true}, {dataTwo, false}} {
		got, control := owner.nextWriteLocked()
		if got != want.request || control != want.control {
			t.Fatalf("schedule %d = %p/%t, want %p/%t", index, got, control, want.request, want.control)
		}
	}
}

func TestClosedSourceChannelsCancelQueuedOpenPreservesSibling(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() {
		frame, err := ardp.ReadFrame(peer)
		if err == nil && (frame.Kind != ardp.KindOpen || frame.Lane != 1) {
			err = errors.New("first OPEN differs")
		}
		opened <- err
	}()
	first, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	blocked := make(chan error, 1)
	go func() { _, err := first.Write([]byte{7}); blocked <- err }()
	waitSourceChannelState(t, owner, func() bool { return owner.active != nil && owner.active.lane == first })
	canceled, cancel := context.WithCancel(t.Context())
	refused := make(chan error, 1)
	go func() {
		_, err := owner.open(canceled, sourceIssuerOpen(end), time.Now().Add(5*time.Second))
		refused <- err
	}()
	waitSourceChannelState(t, owner, func() bool { return len(owner.controls) == 1 })
	cancel()
	select {
	case err := <-refused:
		if err == nil {
			t.Fatal("canceled queued OPEN succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled OPEN waited for sibling writer")
	}
	finished := make(chan error, 1)
	go func() {
		for _, expected := range []ardp.Frame{
			{Kind: ardp.KindBytes, Lane: 1}, {Kind: ardp.KindOpen, Lane: 5}, {Kind: ardp.KindClose, Lane: 5},
		} {
			frame, err := ardp.ReadFrame(peer)
			if err != nil {
				finished <- err
				return
			}
			if frame.Kind != expected.Kind || frame.Lane != expected.Lane {
				finished <- errors.New("unemitted OPEN produced cleanup frames or disturbed sibling")
				return
			}
		}
		finished <- nil
	}()
	if err := <-blocked; err != nil {
		t.Fatal(err)
	}
	next, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestClosedSourceChannelsDeadlineAffectsOnlyItsWaitingReader(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	received := make(chan error, 1)
	go func() {
		for range 2 {
			_, err := ardp.ReadFrame(peer)
			if err != nil {
				received <- err
				return
			}
		}
		received <- nil
	}()
	first, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
	if err := first.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var data [1]byte
	if _, err := first.Read(data[:]); err == nil {
		t.Fatal("expired child read succeeded")
	}
	sent := make(chan error, 1)
	go func() {
		sent <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: second.id, Body: []byte{9}})
	}()
	if count, err := second.Read(data[:]); err != nil || count != 1 || data[0] != 9 {
		t.Fatalf("sibling read affected: %d / %v", count, err)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
}

func TestClosedSourceChannelsCloseCancelsQueuedPayloadBeforeSibling(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	received := make(chan error, 1)
	go func() {
		for range 2 {
			if _, err := ardp.ReadFrame(peer); err != nil {
				received <- err
				return
			}
		}
		received <- nil
	}()
	first, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
	blocked := make(chan error, 1)
	go func() { _, err := first.Write([]byte{1}); blocked <- err }()
	waitSourceChannelState(t, owner, func() bool { return owner.active != nil && owner.active.lane == first })
	queued := make(chan error, 1)
	go func() { _, err := second.Write([]byte{2}); queued <- err }()
	waitSourceChannelState(t, owner, func() bool { return len(owner.data) == 1 })
	closed := make(chan error, 1)
	go func() { closed <- second.Close() }()
	select {
	case err := <-queued:
		if err == nil {
			t.Fatal("closed lane emitted queued payload")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("closed lane retained queued writer behind sibling")
	}
	// Peer remains stalled: the required terminal CLOSE cannot be delivered.
	// Cleanup must retire and join the physical owner within its own bound.
	select {
	case err := <-closed:
		if err == nil {
			t.Fatal("undelivered terminal cleanup reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup waited for sibling lease")
	}
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("failed cleanup left parent workers running")
	}
	if err := <-blocked; err == nil {
		t.Fatal("retired parent writer succeeded")
	}
	owner.mu.Lock()
	retained := owner.queued
	owner.mu.Unlock()
	if retained != 0 {
		t.Fatalf("retired owner retained %d queued bytes", retained)
	}
}
