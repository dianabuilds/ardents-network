//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokens"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// dutyTokenHost adapts the duty context to the token owner's Host seam. Every
// Locked call arrives with owner.mu held, exactly as the pre-extraction
// context methods did; Fail likewise runs under the shared lock.
type dutyTokenHost struct {
	owner *dutyContext
}

func (host dutyTokenHost) ProfileLocked() (state.ClosedProfileView, time.Time, error) {
	return host.owner.permissionProfileLocked()
}

func (host dutyTokenHost) SurfaceRole() admission.AllocationRole {
	if host.owner.surface == broker.Administration {
		return admission.AllocationPublisher
	}
	return admission.AllocationUser
}

func (host dutyTokenHost) Fail(err error) {
	owner := host.owner
	owner.closeErr = errors.Join(owner.closeErr, err)
	owner.closed = true
	owner.endpoint.failDutyContexts(err)
}

func (host dutyTokenHost) Journal() (*tokenjournal.Journal, error) {
	return host.owner.endpoint.tokenJournal()
}

func (host dutyTokenHost) LeaseContext() context.Context {
	return host.owner.lease.Context()
}

func (host dutyTokenHost) SelectBootstrapLocked() (client.ClosedBootstrapSelection, error) {
	return host.owner.selectBootstrapLocked()
}

func (host dutyTokenHost) PrefixCurrent(prefix tokens.Prefix) bool {
	handle, ok := prefix.(*source.Handle)
	return ok && handle.CurrentLocked(&host.owner.source)
}

// prefixRef converts a Source handle to the token owner's Prefix seam. A nil
// handle stays a nil interface so identity comparisons remain exact.
func prefixRef(handle *source.Handle) tokens.Prefix {
	if handle == nil {
		return nil
	}
	return handle
}
