//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

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
