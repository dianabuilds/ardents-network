package node

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

func (running *Initiator) relay(raw, entry, next net.Conn) {
	defer running.work.Done()
	type result struct{ bytes int64 }
	results := make(chan result, 2)
	copyLane := func(destination, source net.Conn) {
		limited := &io.LimitedReader{R: source, N: int64(running.plan.RelayByteLimit)}
		count, _ := io.CopyBuffer(destination, limited, make([]byte, 32<<10))
		results <- result{bytes: count}
	}
	go copyLane(entry, next)
	go copyLane(next, entry)
	first := <-results
	_ = raw.Close()
	_ = next.Close()
	second := <-results
	running.mu.Lock()
	running.usage.RelayedBytes += uint64(first.bytes + second.bytes)
	running.usage.CompletedRelays++
	running.mu.Unlock()
	running.releaseRelay(raw, next)
}

// Drain closes admission, cancels every remaining handshake and relay, and
// joins all duty-owned work before the declared work safety lease expires.
func (running *Initiator) Drain(ctx context.Context) error {
	if running == nil || ctx == nil {
		return errors.New("Initiator duty is unavailable")
	}
	running.Stop()
	running.mu.Lock()
	connections := make([]net.Conn, 0, len(running.pre)+len(running.active))
	for connection := range running.pre {
		connections = append(connections, connection)
	}
	for connection := range running.active {
		connections = append(connections, connection)
	}
	running.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
	done := make(chan struct{})
	go func() { running.work.Wait(); close(done) }()
	timer := time.NewTimer(running.plan.DrainTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("Initiator drain exceeded its Work Safety Lease")
	}
}

func (running *Initiator) closePreAdmissionLocked() []net.Conn {
	connections := make([]net.Conn, 0, len(running.pre))
	for connection := range running.pre {
		connections = append(connections, connection)
	}
	return connections
}

// Close is the explicit shutdown form for callers that do not need a separate
// cancellation deadline.
func (running *Initiator) Close() error { return running.Drain(context.Background()) }
