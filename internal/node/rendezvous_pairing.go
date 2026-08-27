package node

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (running *Rendezvous) register(leg *rendezvousLeg) bool {
	running.mu.Lock()
	if running.draining {
		running.mu.Unlock()
		return false
	}
	existing := running.waiting[leg.binding.AttachmentID]
	if existing == nil {
		delete(running.pre, leg.pending)
		running.waiting[leg.binding.AttachmentID] = leg
		running.mu.Unlock()
		running.work.Add(1)
		go running.expire(leg)
		return true
	}
	if existing.binding.SenderRole == leg.binding.SenderRole {
		running.usage.DuplicateSideRejected++
		running.mu.Unlock()
		return false
	}
	select {
	case running.pairs <- struct{}{}:
	default:
		delete(running.waiting, leg.binding.AttachmentID)
		<-running.waitingCap
		existing.stop()
		running.usage.WaitingRefused++
		running.mu.Unlock()
		return false
	}
	delete(running.waiting, leg.binding.AttachmentID)
	delete(running.pre, existing.pending)
	delete(running.pre, leg.pending)
	<-running.waitingCap
	<-running.waitingCap
	existing.stopDone()
	running.active[existing.connection], running.active[leg.connection] = struct{}{}, struct{}{}
	preAdmission := []io.Closer(nil)
	if len(running.pairs) == cap(running.pairs) {
		preAdmission = running.closePreAdmissionLocked()
	}
	running.work.Add(1)
	running.mu.Unlock()
	for _, connection := range preAdmission {
		_ = connection.Close()
	}
	go running.pump(existing, leg)
	return true
}

func (running *Rendezvous) expire(leg *rendezvousLeg) {
	defer running.work.Done()
	timer := time.NewTimer(leg.binding.NotAfter.Sub(running.plan.now()))
	defer timer.Stop()
	select {
	case <-leg.done:
		return
	case <-timer.C:
	}
	running.mu.Lock()
	if running.waiting[leg.binding.AttachmentID] != leg {
		running.mu.Unlock()
		return
	}
	delete(running.waiting, leg.binding.AttachmentID)
	<-running.waitingCap
	running.usage.Expired++
	leg.stopDone()
	running.mu.Unlock()
	_ = leg.connection.Close()
}

func (running *Rendezvous) pump(first, second *rendezvousLeg) {
	defer running.work.Done()
	type result struct{ bytes int64 }
	results := make(chan result, 2)
	copyLane := func(destination, source route.Carrier) {
		limited := &io.LimitedReader{R: source, N: int64(running.plan.PairByteLimit)}
		count, _ := io.CopyBuffer(destination, limited, make([]byte, 32<<10))
		results <- result{bytes: count}
	}
	go copyLane(first.connection, second.connection)
	go copyLane(second.connection, first.connection)
	firstResult := <-results
	_ = first.connection.Close()
	_ = second.connection.Close()
	secondResult := <-results
	running.mu.Lock()
	delete(running.active, first.connection)
	delete(running.active, second.connection)
	<-running.pairs
	running.usage.RelayedBytes += uint64(firstResult.bytes)
	running.usage.RelayedBytes += uint64(secondResult.bytes)
	running.usage.CompletedPairs++
	running.mu.Unlock()
}

// Drain closes admission first, signals every pre-admission leg, and joins all
// listener, handshake, expiry, and pump work within the declared duty bound.
func (running *Rendezvous) Drain(ctx context.Context) error {
	if running == nil || ctx == nil {
		return errors.New("Rendezvous duty is unavailable")
	}
	running.Stop()
	var preAdmission, active []io.Closer
	running.mu.Lock()
	for connection := range running.pre {
		preAdmission = append(preAdmission, connection)
	}
	for attachment, leg := range running.waiting {
		delete(running.waiting, attachment)
		<-running.waitingCap
		leg.stopDone()
		preAdmission = append(preAdmission, leg.connection)
	}
	for connection := range running.active {
		active = append(active, connection)
	}
	running.mu.Unlock()
	for _, connection := range append(preAdmission, active...) {
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
		return errors.New("Rendezvous drain exceeded its Work Safety Lease")
	}
}

func (running *Rendezvous) closePreAdmissionLocked() []io.Closer {
	connections := make([]io.Closer, 0, len(running.pre)+len(running.waiting))
	for connection := range running.pre {
		connections = append(connections, connection)
	}
	for attachment, leg := range running.waiting {
		delete(running.waiting, attachment)
		<-running.waitingCap
		leg.stopDone()
		connections = append(connections, leg.connection)
	}
	return connections
}

func (leg *rendezvousLeg) stopDone() { leg.doneOnce.Do(func() { close(leg.done) }) }

func (leg *rendezvousLeg) stop() {
	leg.stopDone()
	_ = leg.connection.Close()
}

// Close is the explicit shutdown form for callers that do not need a separate
// cancellation deadline.
func (running *Rendezvous) Close() error { return running.Drain(context.Background()) }
