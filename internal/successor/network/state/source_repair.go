package state

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// ErrSourceRepairUnavailable reports that the retained root is not in the one
// repairable class: a defunct original serving invocation whose sole unresolved
// obligation is the retained serving producer. Every other uncertainty refuses.
var ErrSourceRepairUnavailable = errors.New("retained Source producer repair is unavailable")

// RepairRetainedSourceProducer clears the unresolved serving producer left by an
// original Source invocation that died before its joined completion (class G of
// the recovery contract). It is the single bounded domain-owned repair admitted
// by the retained-only platform path; it is neither a retained runtime opening
// (which refuses the producer) nor a joined Finish (which requires a live joined
// State). It creates nothing, resets no floor, adopts no uncertain process and
// deletes no history: it removes only the serving producer obligation record.
//
// The repair succeeds only when every precondition holds, fail-closed:
//   - the exclusive State-root lease is acquirable (no live State holder) and the
//     physical retained structure verifies (durable.OpenRetained);
//   - the durable generation chain, distribution journal and floors are
//     consistent and the wave/exposure guard resolves (the same semantic
//     admission a retained opening performs, including its monotonic floor
//     auto-repair), so no partial effect survives;
//   - the caller proves the original platform service is inactive with no live
//     main/control process (verifyOriginalInactive); lease release alone is not
//     completion evidence;
//   - the serving producer is the only unresolved obligation: it is retained,
//     and no active cycle journal or exposure mismatch remains (the wave guard
//     above refuses those).
//
// Any uncertainty refuses with the original bytes preserved. A successful repair
// joins the duty operation before returning; a subsequent serve-retained is a new
// independently checked invocation over the same histories.
func RepairRetainedSourceProducer(input Config, verifyOriginalInactive func(context.Context) error) (resultErr error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return err
	}
	if !resolved.sourceInfo.Serving {
		return ErrSourceRepairUnavailable
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	// The exclusive lease acquired here proves no live State holder; the physical
	// retained structure is verified twice by OpenRetained. Nothing is created.
	storage, err := durable.OpenRetained(resolved.root, storageLimits())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, storage.Close()) }()

	workContext, workCancel := context.WithCancel(context.Background())
	defer workCancel()
	s := &networkState{config: resolved, storage: storage, workContext: workContext, workCancel: workCancel}

	// Consistency admission identical to a retained opening, minus startSource:
	// chain + journal + floors verify (with monotonic floor auto-repair) and the
	// wave/exposure guard must resolve. An active cycle journal or exposure
	// mismatch refuses here, so the producer is the only unresolved obligation.
	current, err := loadCurrent(resolved, storage)
	if err != nil {
		return err
	}
	s.current = current
	if current == nil {
		return ErrSourceRepairUnavailable
	}
	if err := s.loadDistributionState(); err != nil {
		return err
	}
	if err := s.recoverSourceWaveGuard(); err != nil {
		return err
	}
	if s.distribution.cycleActive {
		return &RecoveryRequiredError{Reason: "active Source journal has no joined completion to repair"}
	}
	// Native process-death evidence: the platform service must be inactive with no
	// live main/control process. Supplied by the operator command; never inferred
	// from lease release, elapsed time or exit status.
	if verifyOriginalInactive == nil {
		return ErrSourceRepairUnavailable
	}
	if err := verifyOriginalInactive(workContext); err != nil {
		return err
	}
	if resolved.permitWork != nil {
		if err := resolved.permitWork(); err != nil {
			return err
		}
	}
	return s.repairRetainedSourceProducer()
}

// repairRetainedSourceProducer confirms the serving producer is retained (the
// obligation to clear) and removes only that record. The Claim probe refuses
// without side effect when the producer is retained; if it is absent the probe
// would claim it, so the claim is immediately removed and the repair refuses
// (there is no unresolved producer obligation to repair). The duty operation
// lease excludes a concurrent claim throughout.
func (s *networkState) repairRetainedSourceProducer() error {
	roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
	if err != nil {
		return err
	}
	producer := sourceProducer("server", s.config.root)
	duties := []duty.Duty{sourceServerDuty(*s.current)}
	claimErr := roles.Claim(producer, duties)
	if claimErr == nil {
		// No retained producer: restore the probe claim and refuse. This is not the
		// repairable class; a clean root relaunches without repair.
		return errors.Join(ErrSourceRepairUnavailable, roles.Remove(producer), roles.Close())
	}
	if !errors.Is(claimErr, duty.ErrLocalRoleProducerRetained) {
		return errors.Join(claimErr, roles.Close())
	}
	return errors.Join(roles.Remove(producer), roles.Close())
}
