package state

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// ErrJoinRepairUnavailable denotes a history outside the bounded JOIN
// participation-repair class.
var ErrJoinRepairUnavailable = errors.New("retained JOIN producer repair is unavailable")

// A JOIN repair holds both existing leases until the native adapter's final
// observation and the one participation commit. It never starts a State, joins
// an imaginary completion, repairs floors or resumes an interrupted wave. The
// original Node producer identity is random, so the repair discovers the sole
// live node-duty fence and binds it to the current generation. Each JOIN pair
// participant owns its own participation root and its own sole fence; repair is
// per-invocation, never a shared pair effect.
type joinRepair struct {
	config   config
	state    *durable.RetainedReader
	producer [32]byte
	roles    interface {
		Remove([32]byte) error
		Close() error
	}
}

func openJoinRepair(ctx context.Context, resolved config) (_ *joinRepair, result error) {
	if ctx == nil || ctx.Err() != nil || resolved.localRoles == "" {
		if ctx != nil {
			return nil, errors.Join(ErrJoinRepairUnavailable, ctx.Err())
		}
		return nil, ErrJoinRepairUnavailable
	}
	r := &joinRepair{config: resolved}
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
		return nil, errors.Join(ErrJoinRepairUnavailable, err)
	}
	// Bind the discovered fence to a genuine current-generation member. The
	// node-duty producer is claimed with the member's NodeID and FamilyID, not
	// the epoch primary snapshot identity; a substituted or foreign record (no
	// matching accepted Candidate, or a differing family digest) cannot be
	// abandoned by this repair.
	if obligation.Class != "node-duty" || obligation.State != "live" || !nodeDutyMember(current, obligation) {
		return nil, errors.Join(ErrJoinRepairUnavailable, errors.New("retained node-duty obligation differs from the current generation"))
	}
	r.producer = producer
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *joinRepair) checkHistory() (*epoch.Decision, error) {
	current, journal, err := readRetainedHistory(r.config, r.state)
	if err != nil {
		return nil, err
	}
	now, err := trustedNow(r.config, journal)
	if err != nil || now.Before(current.Header.ValidFrom) || !now.Before(current.Snapshot.ValidUntil) {
		return nil, errors.Join(ErrJoinRepairUnavailable, err)
	}
	// Participation-only repair cannot resolve outstanding acquisition/exposure
	// work; spend obligations are never repairable here.
	if journal.cycleActive || journal.pendingDigest != ([32]byte{}) || len(journal.history) != 0 {
		return nil, ErrJoinRepairUnavailable
	}
	return current, nil
}

func (r *joinRepair) close() error {
	var err error
	if r.roles != nil {
		err = r.roles.Close()
	}
	if r.state != nil {
		err = errors.Join(err, r.state.Close())
	}
	return err
}
