//go:build linux

package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// This scheduling regression uses the real channel reader, writer and traffic
// accounting. Network admission and token validity are covered separately.
func TestClosedSourceQueuedRefillCancellation(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) { testClosedSourceQueuedRefillCancellation(t, timeout) })
	}
}

func testClosedSourceQueuedRefillCancellation(t *testing.T, timeout bool) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	traffic := make(chan error, 1)
	go func() {
		var err error
		for range closedRefillThreshold / ardp.MaximumBodySize {
			frame, readErr := ardp.ReadFrame(peer)
			err = readErr
			if err != nil {
				break
			}
			if frame.Kind != ardp.KindBytes {
				err = io.ErrUnexpectedEOF
				break
			}
			err = ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindCredit, Lane: lane.id, Body: binary.BigEndian.AppendUint32(nil, uint32(len(frame.Body)))})

		}
		traffic <- err
	}()
	// Send genuine traffic; no synthetic transferred counter or threshold waiver.
	if _, err := lane.Write(make([]byte, closedRefillThreshold)); err != nil {
		t.Fatal(err)
	}
	if err := <-traffic; err != nil {
		t.Fatal(err)
	}
	blocked := make(chan error, 1)
	go func() { _, err := lane.Write([]byte{7}); blocked <- err }()
	waitSourceChannelState(t, owner, func() bool { return owner.active != nil })
	ctx, cancel := context.WithCancel(t.Context())
	if timeout {
		cancel()
		ctx, cancel = context.WithTimeout(t.Context(), 40*time.Millisecond)
	}
	defer cancel()
	refill := make(chan error, 1)
	go func() {
		refill <- owner.replenish(ctx, ardp.Hello{ChannelNonce: [32]byte{1}, Deadline: end}, func(ardp.Hello, uint8) ([]byte, error) { return make([]byte, 354), nil })
	}()
	waitSourceChannelState(t, owner, func() bool { return len(owner.controls) == 1 })
	if !timeout {
		cancel()
	}
	select {
	case err := <-refill:
		want := context.Canceled
		if timeout {
			want = context.DeadlineExceeded
		}
		if !errors.Is(err, want) {
			t.Fatalf("refill cancellation: %v", err)
		}
	case <-time.After(time.Second):
		owner.fail(context.Canceled)
		<-refill
		t.Fatal("queued refill ignored operation cancellation")
	}
	select {
	case err := <-blocked:
		t.Fatalf("sibling was interrupted: %v", err)
	default:
	}
	frame, err := ardp.ReadFrame(peer)
	if err != nil || frame.Kind != ardp.KindBytes {
		t.Fatalf("sibling frame: %+v %v", frame, err)
	}
	if err := <-blocked; err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	pending, queued := owner.refill != nil, owner.queued
	owner.mu.Unlock()
	if pending || queued != 0 {
		t.Fatalf("canceled refill retained witness or queue: %t %d", pending, queued)
	}
	if err := peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if frame, err := ardp.ReadFrame(peer); err == nil {
		t.Fatalf("late canceled frame: %+v", frame)
	}
}

func TestClosedRoleQueuedRefillCancellation(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			local, peer := net.Pipe()
			end := time.Now().Add(time.Minute)
			stream := newClosedRoleChildStream(local, end, local.Close, nil)
			t.Cleanup(func() { _ = stream.Close(); _ = peer.Close() })
			sibling := make(chan error, 1)
			go func() { _, err := stream.Write([]byte{7}); sibling <- err }()
			header := make([]byte, ardp.HeaderSize)
			if _, err := io.ReadFull(peer, header); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			if timeout {
				ctx, cancel = context.WithTimeout(t.Context(), 40*time.Millisecond)
			}
			defer cancel()
			refill := make(chan error, 1)
			go func() { refill <- stream.replenish(ctx, refillCancellationFrame()) }()
			if !timeout {
				cancel()
			}
			select {
			case err := <-refill:
				want := context.Canceled
				if timeout {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) {
					t.Fatalf("refill: %v", err)
				}
			case <-time.After(time.Second):
				_ = local.Close()
				<-refill
				t.Fatal("inner queued refill ignored caller cancellation")
			}
			select {
			case err := <-sibling:
				t.Fatalf("sibling interrupted: %v", err)
			default:
			}
			frame, err := ardp.ReadFrame(io.MultiReader(bytes.NewReader(header), peer))
			if err != nil || frame.Kind != ardp.KindBytes {
				t.Fatalf("sibling: %+v %v", frame, err)
			}
			if err := <-sibling; err != nil {
				t.Fatal(err)
			}
			stream.mu.Lock()
			pending, terminal := stream.refill != nil, stream.terminal
			stream.mu.Unlock()
			if pending || terminal != nil {
				t.Fatalf("queued refill damaged parent: %t %v", pending, terminal)
			}
			_ = peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
			if frame, err := ardp.ReadFrame(peer); err == nil {
				t.Fatalf("late inner ADMIT: %+v", frame)
			}
		})
	}
}

func refillCancellationFrame() ardp.Frame {
	body := make([]byte, 355)
	body[0] = 2
	return ardp.Frame{Kind: ardp.KindAdmit, Body: body}
}

func TestClosedRoleActiveRefillCancellationJoinsFrame(t *testing.T) {
	local, peer := net.Pipe()
	end := time.Now().Add(time.Minute)
	stream := newClosedRoleChildStream(local, end, local.Close, nil)
	t.Cleanup(func() { _ = stream.Close(); _ = peer.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	refill := make(chan error, 1)
	go func() { refill <- stream.replenish(ctx, refillCancellationFrame()) }()
	// Consume only its real header; its body is physically blocked.
	header := make([]byte, ardp.HeaderSize)
	if _, err := io.ReadFull(peer, header); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-refill:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active cancellation: %v", err)
		}
	case <-time.After(time.Second):
		_ = local.Close()
		<-refill
		t.Fatal("active inner refill did not join interruption")
	}
	select {
	case <-stream.done:
	default:
		t.Fatal("reader survived partial refill")
	}
	if n, err := peer.Read(make([]byte, 355)); n != 0 || err == nil {
		t.Fatalf("late refill body: %d %v", n, err)
	}
	stream.mu.Lock()
	active, pending, failed := stream.physicalWriting, stream.refill != nil, stream.physicalWriteFailed
	stream.mu.Unlock()
	if active || pending || !failed {
		t.Fatalf("inner framing witness: %t %t %t", active, pending, failed)
	}
}

func TestClosedSourceActiveRefillCancellationJoinsFrame(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	// Traffic is already covered at the public threshold boundary above; this
	// focused framing test isolates cancellation after a real ADMIT header.
	owner.mu.Lock()
	owner.transferred = closedRefillThreshold
	owner.mu.Unlock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	refill := make(chan error, 1)
	go func() {
		refill <- owner.replenish(ctx, ardp.Hello{ChannelNonce: [32]byte{1}, Deadline: end}, func(ardp.Hello, uint8) ([]byte, error) { return make([]byte, 354), nil })
	}()
	if _, err := io.ReadFull(peer, make([]byte, ardp.HeaderSize)); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-refill:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active outer cancellation: %v", err)
		}
	case <-time.After(time.Second):
		owner.fail(context.Canceled)
		<-refill
		t.Fatal("outer refill did not join active write")
	}
	select {
	case <-owner.done:
	default:
		t.Fatal("outer owner workers survived partial refill")
	}
	if n, err := peer.Read(make([]byte, 355)); n != 0 || err == nil {
		t.Fatalf("late outer body: %d %v", n, err)
	}
	owner.mu.Lock()
	active, pending, queued := owner.active != nil, owner.refill != nil, owner.queued
	owner.mu.Unlock()
	if active || pending || queued != 0 {
		t.Fatalf("outer join state: %t %t %d", active, pending, queued)
	}
}

func TestClosedSourceAlreadyCanceledRefillDoesNotPresent(t *testing.T) {
	owner, _, end := sourceChannelsFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	err := owner.replenish(ctx, ardp.Hello{ChannelNonce: [32]byte{1}, Deadline: end}, func(ardp.Hello, uint8) ([]byte, error) { called = true; return nil, nil })
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("already canceled: %v presenter=%t", err, called)
	}
}

func TestClosedSourceRepeatedRefillWaiterCancels(t *testing.T) {
	prefix, _ := sourceResolutionSelectionFixture(t)
	prefix.refillOnce.Do(func() { prefix.refillGate = make(chan struct{}, 1) })
	prefix.refillGate <- struct{}{}
	defer func() { <-prefix.refillGate }()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	called := false
	err := prefix.Replenish(ctx, func(ardp.Hello, uint8) ([]byte, error) { called = true; return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatalf("waiting repeated refill: %v presenter=%t", err, called)
	}
}
