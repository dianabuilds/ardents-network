package route

import (
	"errors"
	"sync"
	"time"
)

const (
	closedBootstrapLaneBytes   = 128 << 10
	closedBootstrapLaneLife    = 10 * time.Second
	closedBootstrapAdjacent    = 4
	closedBootstrapDuty        = 16
	closedBootstrapQueueBytes  = 256 << 10
	closedBootstrapOutputRate  = 1 << 20
	closedBootstrapOutputBurst = 128 << 10
)

// ClosedBootstrapController owns one duty's aggregate target-free bootstrap
// allocation. Its adjacency key is an authenticated local carrier fact, not a
// source address, endpoint identity, or permission identifier.
type ClosedBootstrapController struct {
	mu       sync.Mutex
	clock    func() time.Time
	adjacent map[[32]byte]uint8
	leases   map[*ClosedBootstrapLease]struct{}
	queued   uint64
	tokens   uint64
	refilled time.Time
}

// ClosedBootstrapLease is one short-lived target-free bootstrap allocation.
// It cannot become a private lane; the lane owner must separately admit it.
type ClosedBootstrapLease struct {
	controller *ClosedBootstrapController
	adjacency  [32]byte
	deadline   time.Time
	used       uint64
	queued     uint64
	released   bool
}

// NewClosedBootstrapController creates the finite shared duty governor.
func NewClosedBootstrapController(clock func() time.Time) (*ClosedBootstrapController, error) {
	if clock == nil || clock().IsZero() {
		return nil, errors.New("closed bootstrap clock is invalid")
	}
	now := clock().UTC()
	return &ClosedBootstrapController{clock: clock, adjacent: make(map[[32]byte]uint8), leases: make(map[*ClosedBootstrapLease]struct{}),
		tokens: closedBootstrapOutputBurst, refilled: now}, nil
}

// Admit allocates one bootstrap lane for at most ten seconds and never beyond
// the caller's earlier State/operation deadline.
func (controller *ClosedBootstrapController) Admit(adjacency [32]byte, deadline time.Time) (*ClosedBootstrapLease, error) {
	if controller == nil || adjacency == [32]byte{} || deadline.IsZero() || deadline != deadline.UTC() {
		return nil, errors.New("closed bootstrap admission is invalid")
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	now := controller.clock().UTC()
	controller.reap(now)
	if !now.Before(deadline) || len(controller.leases) >= closedBootstrapDuty || controller.adjacent[adjacency] >= closedBootstrapAdjacent {
		return nil, errors.New("closed bootstrap is unavailable")
	}
	if bounded := now.Add(closedBootstrapLaneLife); deadline.After(bounded) {
		deadline = bounded
	}
	lease := &ClosedBootstrapLease{controller: controller, adjacency: adjacency, deadline: deadline}
	controller.leases[lease] = struct{}{}
	controller.adjacent[adjacency]++
	return lease, nil
}

// Queue reserves bounded ciphertext work before a lane writes it. It prevents
// one peer from manufacturing an unbounded duty queue.
func (lease *ClosedBootstrapLease) Queue(bytes uint64) error {
	if bytes == 0 {
		return errors.New("closed bootstrap queue increment is invalid")
	}
	return lease.withLive(func(controller *ClosedBootstrapController) error {
		if lease.queued+bytes > closedBootstrapLaneBytes || controller.queued+bytes > closedBootstrapQueueBytes {
			return errors.New("closed bootstrap queue is exhausted")
		}
		lease.queued += bytes
		controller.queued += bytes
		return nil
	})
}

// Dequeue releases only already reserved queue bytes after the lane's actual
// consumer has taken them.
func (lease *ClosedBootstrapLease) Dequeue(bytes uint64) error {
	if bytes == 0 {
		return errors.New("closed bootstrap queue decrement is invalid")
	}
	return lease.withLive(func(controller *ClosedBootstrapController) error {
		if bytes > lease.queued {
			return errors.New("closed bootstrap queue decrement exceeds reservation")
		}
		lease.queued -= bytes
		controller.queued -= bytes
		return nil
	})
}

// Send accounts one actual bootstrap output write against both the individual
// lane and the shared 1 MiB/minute, 128 KiB burst budget.
func (lease *ClosedBootstrapLease) Send(bytes uint64) error {
	if bytes == 0 {
		return errors.New("closed bootstrap output is invalid")
	}
	return lease.withLive(func(controller *ClosedBootstrapController) error {
		controller.refill(controller.clock().UTC())
		if lease.used+bytes > closedBootstrapLaneBytes || bytes > controller.tokens {
			return errors.New("closed bootstrap output is exhausted")
		}
		lease.used += bytes
		controller.tokens -= bytes
		return nil
	})
}

// Release returns all finite lane reservations. It is idempotent so every
// cancellation, parse failure, and normal terminal path can defer it safely.
func (lease *ClosedBootstrapLease) Release() {
	if lease == nil || lease.controller == nil {
		return
	}
	lease.controller.mu.Lock()
	defer lease.controller.mu.Unlock()
	lease.controller.release(lease)
}

func (lease *ClosedBootstrapLease) withLive(operation func(*ClosedBootstrapController) error) error {
	if lease == nil || lease.controller == nil {
		return errors.New("closed bootstrap lane is unavailable")
	}
	controller := lease.controller
	controller.mu.Lock()
	defer controller.mu.Unlock()
	now := controller.clock().UTC()
	controller.reap(now)
	if lease.released {
		return errors.New("closed bootstrap lane is unavailable")
	}
	return operation(controller)
}

func (controller *ClosedBootstrapController) reap(now time.Time) {
	for lease := range controller.leases {
		if !now.Before(lease.deadline) {
			controller.release(lease)
		}
	}
}

func (controller *ClosedBootstrapController) release(lease *ClosedBootstrapLease) {
	if lease == nil || lease.released {
		return
	}
	lease.released = true
	delete(controller.leases, lease)
	if controller.adjacent[lease.adjacency] <= 1 {
		delete(controller.adjacent, lease.adjacency)
	} else {
		controller.adjacent[lease.adjacency]--
	}
	controller.queued -= lease.queued
	lease.queued = 0
}

func (controller *ClosedBootstrapController) refill(now time.Time) {
	if now.Before(controller.refilled) {
		return
	}
	elapsed := now.Sub(controller.refilled)
	if elapsed >= time.Minute {
		controller.tokens = closedBootstrapOutputBurst
		controller.refilled = now
		return
	}
	add := uint64(elapsed) * closedBootstrapOutputRate / uint64(time.Minute)
	if add > closedBootstrapOutputBurst-controller.tokens {
		controller.tokens = closedBootstrapOutputBurst
	} else {
		controller.tokens += add
	}
	controller.refilled = now
}
