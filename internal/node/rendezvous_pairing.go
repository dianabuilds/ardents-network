package node

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (running *rendezvous) register(leg *rendezvousLeg) bool {
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
		existing.stopDone()
		running.cleanup.record(existing.connection.Close())
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
		running.cleanup.record(connection.Close())
	}
	go running.pump(existing, leg)
	return true
}

func (running *rendezvous) expire(leg *rendezvousLeg) {
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
	running.cleanup.record(leg.connection.Close())
}

func (running *rendezvous) pump(first, second *rendezvousLeg) {
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
	running.cleanup.record(first.connection.Close())
	running.cleanup.record(second.connection.Close())
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

// Drain closes admission and pre-admission work first, lets accepted pairs
// finish inside the declared duty bound, then closes any pair still active at
// that boundary before joining every owned worker.
func (running *rendezvous) Drain(ctx context.Context) error {
	if running == nil || ctx == nil {
		return errors.New("rendezvous duty is unavailable")
	}
	running.Stop()
	var preAdmission []io.Closer
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
	running.mu.Unlock()
	for _, connection := range preAdmission {
		running.cleanup.record(connection.Close())
	}
	done := make(chan struct{})
	go func() { running.work.Wait(); close(done) }()
	timer := time.NewTimer(running.plan.DrainTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return running.cleanup.result()
	case <-ctx.Done():
		running.closeActivePairs()
		<-done
		return errors.Join(running.cleanup.result(), ctx.Err())
	case <-timer.C:
		running.closeActivePairs()
		<-done
		return running.cleanup.result()
	}
}

func (running *rendezvous) closeActivePairs() {
	running.mu.Lock()
	active := make([]io.Closer, 0, len(running.active))
	for connection := range running.active {
		active = append(active, connection)
	}
	running.mu.Unlock()
	for _, connection := range active {
		running.cleanup.record(connection.Close())
	}
}

func (running *rendezvous) closePreAdmissionLocked() []io.Closer {
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

// Close is the explicit shutdown form for callers that do not need a separate
// cancellation deadline.
func (running *rendezvous) Close() error { return running.Drain(context.Background()) }
