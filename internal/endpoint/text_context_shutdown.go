//go:build linux

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// retainTextContext keeps pending worker cleanup in Endpoint's shutdown tree.
// The Broker lease still supplies authorization and the finite admission bound.
func (endpoint *endpoint) retainTextContext(owner *textContext) error {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	if endpoint.textClosed || owner.lease.Context().Err() != nil {
		return errors.New("text context Endpoint is closed")
	}
	count := 0
	for retained := range endpoint.textContexts {
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
	if endpoint.textContexts == nil {
		endpoint.textContexts = make(map[*textContext]struct{})
	}
	endpoint.textContexts[owner] = struct{}{}
	return nil
}

func (endpoint *endpoint) releaseTextContext(owner *textContext, err error) {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	// Retain only the first failure, not an unbounded history of closed contexts.
	if endpoint.textErr == nil {
		endpoint.textErr = err
	}
	delete(endpoint.textContexts, owner)
}

func (endpoint *endpoint) closeTextContexts() error {
	endpoint.textMu.Lock()
	endpoint.textClosed = true
	pending := make([]*textContext, 0, len(endpoint.textContexts))
	for owner := range endpoint.textContexts {
		pending = append(pending, owner)
	}
	endpoint.textMu.Unlock()
	// First cancel every owner; a slow cleanup must not delay another revoke.
	for _, owner := range pending {
		owner.lease.Release()
	}
	for _, owner := range pending {
		_ = owner.Close()
	}
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	return endpoint.textErr
}

// failTextContexts closes the existing local cleanup tree before a failed
// reservation is released. No fresh context or previously idle context can
// reuse authority while a worker cgroup may remain live. There is no reset or
// recovery switch; this Endpoint generation remains unavailable for text jobs.
func (endpoint *endpoint) failTextContexts(err error) {
	endpoint.textMu.Lock()
	if endpoint.textErr == nil {
		endpoint.textErr = err
	}
	endpoint.textClosed = true
	pending := make([]*textContext, 0, len(endpoint.textContexts))
	for owner := range endpoint.textContexts {
		pending = append(pending, owner)
	}
	endpoint.textMu.Unlock()
	for _, owner := range pending {
		owner.lease.Release()
	}
}

func (endpoint *endpoint) textAvailable() bool {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	return !endpoint.textClosed
}
