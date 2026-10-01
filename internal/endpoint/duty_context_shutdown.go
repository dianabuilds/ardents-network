//go:build linux

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// retainDutyContext keeps pending worker cleanup in Endpoint's shutdown tree.
// The Broker lease still supplies authorization and the finite admission bound.
func (endpoint *endpoint) retainDutyContext(owner *dutyContext) error {
	endpoint.dutyMu.Lock()
	defer endpoint.dutyMu.Unlock()
	if endpoint.dutyClosed || owner.lease.Context().Err() != nil {
		return errors.New("text context Endpoint is closed")
	}
	count := 0
	for retained := range endpoint.dutyContexts {
		if retained.surface == owner.surface {
			count++
		}
	}
	limit := 64
	if owner.surface == broker.Administration {
		limit = 6
	}
	if count >= limit {
		return errors.New("text context cleanup capacity is exhausted")
	}
	if endpoint.dutyContexts == nil {
		endpoint.dutyContexts = make(map[*dutyContext]struct{})
	}
	endpoint.dutyContexts[owner] = struct{}{}
	return nil
}

func (endpoint *endpoint) releaseDutyContext(owner *dutyContext, err error) {
	endpoint.dutyMu.Lock()
	defer endpoint.dutyMu.Unlock()
	// Retain only the first failure, not an unbounded history of closed contexts.
	if endpoint.dutyErr == nil {
		endpoint.dutyErr = err
	}
	delete(endpoint.dutyContexts, owner)
}

func (endpoint *endpoint) closeDutyContexts() error {
	endpoint.dutyMu.Lock()
	endpoint.dutyClosed = true
	pending := make([]*dutyContext, 0, len(endpoint.dutyContexts))
	for owner := range endpoint.dutyContexts {
		pending = append(pending, owner)
	}
	endpoint.dutyMu.Unlock()
	// First cancel every owner; a slow cleanup must not delay another revoke.
	for _, owner := range pending {
		owner.lease.Release()
	}
	for _, owner := range pending {
		_ = owner.Close()
	}
	endpoint.dutyMu.Lock()
	defer endpoint.dutyMu.Unlock()
	return endpoint.dutyErr
}

// failDutyContexts closes the existing local cleanup tree before a failed
// reservation is released. No fresh context or previously idle context can
// reuse authority while a worker cgroup may remain live. There is no reset or
// recovery switch; this Endpoint generation remains unavailable for text jobs.
func (endpoint *endpoint) failDutyContexts(err error) {
	endpoint.dutyMu.Lock()
	if endpoint.dutyErr == nil {
		endpoint.dutyErr = err
	}
	endpoint.dutyClosed = true
	pending := make([]*dutyContext, 0, len(endpoint.dutyContexts))
	for owner := range endpoint.dutyContexts {
		pending = append(pending, owner)
	}
	endpoint.dutyMu.Unlock()
	for _, owner := range pending {
		owner.lease.Release()
	}
}

func (endpoint *endpoint) dutyAvailable() bool {
	endpoint.dutyMu.Lock()
	defer endpoint.dutyMu.Unlock()
	return !endpoint.dutyClosed
}
