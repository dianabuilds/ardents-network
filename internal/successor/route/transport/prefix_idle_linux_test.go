//go:build linux

package transport

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Drive only the readiness timer seam. These physical session fixtures grant
// no Network/Admission authority or integrated protected-prefix success.
func TestPrefixIdleReadinessRetiresBeforeJoinedRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	end := time.Now().Add(5 * time.Second)
	entry, interior := newLifecycleConn(false), newLifecycleConn(false)
	queues := &queueBudget{maximum: 4 << 20}
	p := &Prefix{ctx: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), entry: newSession(ctx, entry, end, 32<<20, nil, false, queues, nil), interior: newSession(ctx, interior, end, 32<<20, nil, false, queues, nil)}
	var released atomic.Int32
	p.release = func() error {
		select {
		case <-p.entry.readerDone:
		default:
			t.Error("idle release preceded Entry join")
		}
		select {
		case <-p.interior.readerDone:
		default:
			t.Error("idle release preceded Interior join")
		}
		released.Add(1)
		return nil
	}
	idle := make(chan time.Time, 1)
	go p.watch(func() error { return nil }, idle)
	idle <- time.Now()
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("idle readiness did not retire")
	}
	if released.Load() != 0 {
		t.Fatal("readiness notification released physical capacity")
	}
	err := p.Close()
	if err == nil || !strings.Contains(err.Error(), "idle readiness expired") || released.Load() != 1 {
		t.Fatal("idle joined result lost", err, released.Load())
	}
	if p.Close() != err || released.Load() != 1 {
		t.Fatal("idle close renewed its outcome")
	}
}
