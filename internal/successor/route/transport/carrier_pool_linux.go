//go:build linux

package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// nodePool belongs to one receiving Node. Opening and retirement occupy the
// same directed-pair slot as a ready Carrier. No idle retention or pre-dial is
// needed: the last joined borrower retires its incarnation before replacement.
type nodePool struct {
	mu         sync.Mutex
	entries    map[[32]byte]*pooledCarrier
	closed     bool
	operations sync.WaitGroup
	once       sync.Once
	err        error
}
type pooledCarrier struct {
	pool           *nodePool
	peer           [32]byte
	duty           network.RetainedDuty
	profile        network.ProfileBinding
	session        *session
	changed        chan struct{}
	busy           bool
	refs           uint32
	releaseControl func()
}

func (p *nodePool) acquire(ctx context.Context, a Authority, open func(context.Context) (*session, func(), error)) (*pooledCarrier, error) {
	return p.borrow(ctx, a, func() error { _, err := a.member(); return err }, open)
}

func (p *nodePool) borrow(ctx context.Context, a Authority, validate func() error, open func(context.Context) (*session, func(), error)) (*pooledCarrier, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validate(); err != nil {
			return nil, err
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, errors.New("route Carrier pool retired")
		}
		if p.entries == nil {
			p.entries = make(map[[32]byte]*pooledCarrier)
		}
		current := p.entries[a.Duty.NodeID]
		if current != nil {
			if !current.busy && current.duty == a.Duty && current.profile == a.Profile {
				current.session.mu.Lock()
				live := !current.session.stopped
				current.session.mu.Unlock()
				if live {
					current.refs++
					p.mu.Unlock()
					return current, nil
				}
			}
			wait, s := current.changed, current.session
			p.mu.Unlock()
			// Revalidation and physical interruption never run under pool lock.
			if s != nil && (current.duty != a.Duty || current.profile != a.Profile) {
				s.retire(errors.New("route pooled binding changed"))
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-wait:
			}
			continue
		}
		if len(p.entries) >= 32 {
			p.mu.Unlock()
			return nil, errors.New("route Node Carrier capacity exhausted")
		}
		entry := &pooledCarrier{pool: p, peer: a.Duty.NodeID, duty: a.Duty, profile: a.Profile, changed: make(chan struct{}), busy: true}
		p.entries[entry.peer] = entry
		p.operations.Add(1)
		p.mu.Unlock()
		s, release, err := open(ctx)
		if err == nil {
			err = validate()
		}
		if err == nil {
			err = ctx.Err()
		}
		p.mu.Lock()
		if p.closed && err == nil {
			err = errors.New("route Carrier pool retired during opening")
		}
		if err == nil {
			entry.session, entry.releaseControl, entry.refs, entry.busy = s, release, 1, false
			close(entry.changed)
			entry.changed = make(chan struct{})
			p.mu.Unlock()
			p.operations.Done()
			return entry, nil
		}
		p.mu.Unlock()
		if s != nil {
			err = errors.Join(err, s.Close())
		}
		if release != nil {
			release()
		}
		p.mu.Lock()
		if s != nil {
			p.err = errors.Join(p.err, s.joinedPhysicalFailure())
		}
		delete(p.entries, entry.peer)
		close(entry.changed)
		p.mu.Unlock()
		p.operations.Done()
		return nil, err
	}
}

func (c *pooledCarrier) release() error {
	p := c.pool
	p.mu.Lock()
	c.refs--
	if c.refs != 0 {
		p.mu.Unlock()
		return nil
	}
	c.busy = true
	p.operations.Add(1)
	p.mu.Unlock()
	err := c.session.Close()
	c.releaseControl()
	p.mu.Lock()
	if physical := c.session.joinedPhysicalFailure(); physical != nil {
		p.err = errors.Join(p.err, fmt.Errorf("route outgoing carrier retirement: %w", physical))
	}
	delete(p.entries, c.peer)
	close(c.changed)
	p.mu.Unlock()
	p.operations.Done()
	return err
}

func (p *nodePool) interrupt() {
	p.mu.Lock()
	p.closed = true
	var sessions []*session
	for _, entry := range p.entries {
		if entry.session != nil {
			sessions = append(sessions, entry.session)
		}
	}
	p.mu.Unlock()
	for _, s := range sessions {
		s.retire(nil)
	}
}

// Close follows Receiver's borrower join. It also joins an owned late dial;
// no result can publish after interrupt has sealed pool admission.
func (p *nodePool) Close() error {
	p.once.Do(func() { p.interrupt(); p.operations.Wait() })
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}
