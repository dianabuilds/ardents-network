package state

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
)

// nodeDutyRepairRole carries the static facts that distinguish one node-duty
// role's participation repair from its siblings: the role name used by the
// native root/unit/declaration bindings and the role's refusal sentinel. The
// Forwarding, JOIN, Descriptor and Introduction repairs are one mechanism; the
// role descriptor is the only portable divergence. Source repair is a separate
// deterministic serving-producer mechanism and does not use this descriptor.
type nodeDutyRepairRole struct {
	name        string
	unavailable error
}

// A nodeDutyRepair holds both existing leases until the native adapter's final
// observation and the one participation commit. It never starts a State, joins
// an imaginary completion, repairs floors or resumes an interrupted wave. The
// original Node producer identity is random, so the repair discovers the sole
// live node-duty fence and binds it to the current generation. Each role
// invocation owns its own participation root and sole fence; repair is
// per-invocation, never a shared multi-role effect.
type nodeDutyRepair struct {
	role     nodeDutyRepairRole
	config   config
	state    *durable.RetainedReader
	producer [32]byte
	roles    interface {
		Remove([32]byte) error
		Close() error
	}
}

func openNodeDutyRepair(ctx context.Context, resolved config, role nodeDutyRepairRole) (_ *nodeDutyRepair, result error) {
	if ctx == nil || ctx.Err() != nil || resolved.localRoles == "" {
		if ctx != nil {
			return nil, errors.Join(role.unavailable, ctx.Err())
		}
		return nil, role.unavailable
	}
	r := &nodeDutyRepair{role: role, config: resolved}
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
		return nil, errors.Join(role.unavailable, err)
	}
	// Bind the discovered fence to a genuine current-generation member. The
	// node-duty producer is claimed with the member's NodeID and FamilyID, not
	// the epoch primary snapshot identity; a substituted or foreign record (no
	// matching accepted Candidate, or a differing family digest) cannot be
	// abandoned by this repair.
	if obligation.Class != "node-duty" || obligation.State != "live" || !nodeDutyMember(current, obligation) {
		return nil, errors.Join(role.unavailable, errors.New("retained node-duty obligation differs from the current generation"))
	}
	r.producer = producer
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

// nodeDutyMember reports whether the discovered node-duty obligation belongs to
// an accepted Candidate of the current generation with the exact authenticated
// family digest. The runtime claims the producer with the member's NodeID and
// FamilyID, so the repair binds against the member record rather than the epoch
// primary snapshot identity used by the deterministic Source serving producer.
func nodeDutyMember(current *epoch.Decision, obligation duty.Duty) bool {
	for _, candidate := range current.Candidates {
		if candidate.NodeID == obligation.Identity {
			return candidate.FamilyID == obligation.Family
		}
	}
	return false
}

func (r *nodeDutyRepair) checkHistory() (*epoch.Decision, error) {
	current, journal, err := readRetainedHistory(r.config, r.state)
	if err != nil {
		return nil, err
	}
	now, err := trustedNow(r.config, journal)
	if err != nil || now.Before(current.Header.ValidFrom) || !now.Before(current.Snapshot.ValidUntil) {
		return nil, errors.Join(r.role.unavailable, err)
	}
	// Participation-only repair cannot resolve outstanding acquisition/exposure
	// work; slot claims and spend obligations are never repairable here.
	if journal.CycleActive || journal.PendingDigest != ([32]byte{}) || len(journal.History) != 0 {
		return nil, r.role.unavailable
	}
	return current, nil
}

func (r *nodeDutyRepair) close() error {
	var err error
	if r.roles != nil {
		err = r.roles.Close()
	}
	if r.state != nil {
		err = errors.Join(err, r.state.Close())
	}
	return err
}
