package state

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// ErrForwardingRepairUnavailable denotes a history outside the bounded
// Forwarding participation-repair class.
var ErrForwardingRepairUnavailable = errors.New("retained Forwarding producer repair is unavailable")

// A Forwarding repair holds both existing leases until the native adapter's
// final observation and the one participation commit. It never starts a State,
// joins an imaginary completion, repairs floors or resumes an interrupted wave.
// The original Node producer identity is random, so the repair discovers the
// sole live node-duty fence and binds it to the current generation.
type forwardingRepair struct {
	config   config
	state    *durable.RetainedReader
	producer [32]byte
	roles    interface {
		Remove([32]byte) error
		Close() error
	}
}

func openForwardingRepair(ctx context.Context, resolved config) (_ *forwardingRepair, result error) {
	if ctx == nil || ctx.Err() != nil || resolved.localRoles == "" {
		if ctx != nil {
			return nil, errors.Join(ErrForwardingRepairUnavailable, ctx.Err())
		}
		return nil, ErrForwardingRepairUnavailable
	}
	r := &forwardingRepair{config: resolved}
	defer func() {
		if result != nil {
			result = errors.Join(result, r.close())
		}
	}()
	var err error
	r.state, err = durable.OpenRetainedReader(resolved.root, storageLimits())
	if err != nil {
		return nil, err
	}
	current, err := r.checkHistory()
	if err != nil {
		return nil, err
	}
	roles, err := duty.OpenRetainedOperation(ctx, duty.Config{Root: resolved.localRoles, Clock: resolved.clock})
	if err != nil {
		return nil, err
	}
	r.roles = roles
	producer, obligation, err := roles.CheckNodeProducer()
	if err != nil {
		return nil, errors.Join(ErrForwardingRepairUnavailable, err)
	}
	// Bind the discovered fence to the exact current generation's node identity.
	// A substituted or foreign node-duty record cannot be abandoned by this repair.
	if obligation.Identity != current.Snapshot.NodeID || obligation.Family != sha256.Sum256([]byte(current.Snapshot.DeclaredFamily)) || obligation.Class != "node-duty" || obligation.State != "live" {
		return nil, errors.Join(ErrForwardingRepairUnavailable, errors.New("retained node-duty obligation differs from the current generation"))
	}
	r.producer = producer
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *forwardingRepair) checkHistory() (*epoch.Decision, error) {
	current, journal, err := readRetainedHistory(r.config, r.state)
	if err != nil {
		return nil, err
	}
	now, err := trustedNow(r.config, journal)
	if err != nil || now.Before(current.Header.ValidFrom) || !now.Before(current.Snapshot.ValidUntil) {
		return nil, errors.Join(ErrForwardingRepairUnavailable, err)
	}
	// Participation-only repair cannot resolve outstanding acquisition/exposure
	// work; spend obligations are never repairable here.
	if journal.cycleActive || journal.pendingDigest != ([32]byte{}) || len(journal.history) != 0 {
		return nil, ErrForwardingRepairUnavailable
	}
	return current, nil
}

func (r *forwardingRepair) close() error {
	var err error
	if r.roles != nil {
		err = r.roles.Close()
	}
	if r.state != nil {
		err = errors.Join(err, r.state.Close())
	}
	return err
}
