package state

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

var errSourceRoleCollision = errors.New("source role collides with a Candidate View member")

func (s *networkState) rejectSourceCollisions() error {
	for _, decision := range []*epoch.Decision{s.current, s.pendingDecision} {
		if decision != nil && sourceCollides(s.config.sourceInfo, *decision) {
			return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
		}
	}
	return nil
}

func (s *networkState) rejectDecisionSourceCollisions(decision epoch.Decision) error {
	if sourceCollides(s.config.sourceInfo, decision) {
		return fmt.Errorf("%w: identity, family, or endpoint", errSourceRoleCollision)
	}
	return nil
}

func sourceCollides(info source.Details, decision epoch.Decision) bool {
	if !info.Configured {
		return false
	}
	for index := range info.Identities {
		if epochDecisionCollides(decision, info.Identities[index], info.Families[index], info.EndpointHandles[index]) {
			return true
		}
	}
	return false
}

func epochDecisionCollides(decision epoch.Decision, identity [32]byte, family, endpoint string) bool {
	for _, candidate := range decision.Candidates {
		if candidate.NodeID == identity || candidate.KeyID == identity ||
			candidate.Family == family || candidate.Endpoint == endpoint {
			return true
		}
	}
	return false
}

func (s *networkState) retainSourceExposures(notAfter time.Time) error {
	return s.replaceSourceExposures(notAfter, "exposed")
}

func (s *networkState) holdSourceExposures(notAfter time.Time) error {
	return s.replaceSourceExposures(notAfter, "live")
}

func (s *networkState) replaceSourceExposures(notAfter time.Time, state string) error {
	roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock, Create: true})
	if err != nil {
		return err
	}
	duties := make([]duty.Duty, len(s.config.sourceInfo.Identities))
	for index, identity := range s.config.sourceInfo.Identities {
		duties[index] = duty.Duty{Identity: identity,
			Family: sha256.Sum256([]byte(s.config.sourceInfo.Families[index])),
			Class:  "direct-source", State: state, NotAfter: notAfter}
	}
	if err := roles.Replace(sourceProducer("exposure", s.config.root), duties); err != nil {
		return errors.Join(fmt.Errorf("replace Source exposure: %w", err), roles.Close())
	}
	return roles.Close()
}

// The contact owner calls this only after both contacts have joined and the
// terminal wave state has been committed. The current and pending generations
// may outlive the journal deadline, so their exposure bound takes precedence.
func (s *networkState) releaseSourceWave() error {
	if !s.config.sourceInfo.Configured || s.distribution.cycleActive {
		return nil
	}
	return s.releaseJoinedSourceWave()
}

// Called under the State lock while the Refresh owner is still active.
func (s *networkState) releaseSourceWaveLocked() error {
	if err := s.releaseSourceWave(); err != nil {
		s.terminalErr = fmt.Errorf("release direct Source contact guard: %w", err)
		s.retireStateLocked()
		return s.terminalErr
	}
	return nil
}

func (s *networkState) releaseJoinedSourceWave() error {
	if !s.config.sourceInfo.Configured || s.distribution.cycleID == 0 {
		return nil
	}
	bound := time.Unix(s.distribution.cycleDeadline, 0)
	for _, decision := range []*epoch.Decision{s.current, s.pendingDecision} {
		if decision != nil && decision.Header.ValidUntil.After(bound) {
			bound = decision.Header.ValidUntil
		}
	}
	if bound.After(s.config.clock()) {
		return s.retainSourceExposures(bound)
	}
	roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock, Create: true})
	if err != nil {
		return err
	}
	return errors.Join(roles.Remove(sourceProducer("exposure", s.config.root)), roles.Close())
}

// A reopened root owns no old contacts. Only the durable exposure history can
// tie a previous wave to the configured Source pair; if it disagrees, keep the
// old producer guard intact until the operator restores that plan.
func (s *networkState) recoverSourceWaveGuard() error {
	if !s.distribution.cycleActive && len(s.distribution.history) == 0 {
		return nil
	}
	if !s.config.sourceInfo.Configured {
		if s.distribution.cycleActive {
			return &RecoveryRequiredError{Reason: "active Source journal has no configured Source plan"}
		}
		// A terminal journal has no contact to resume. Offline readers leave
		// its previously committed Duty producer intact without reclaiming it.
		return nil
	}
	for _, exposure := range s.distribution.history {
		if exposure != s.config.sourceInfo.Exposures[0] && exposure != s.config.sourceInfo.Exposures[1] {
			return &RecoveryRequiredError{Reason: "Source exposure history does not match the configured Source plan"}
		}
	}
	return s.releaseJoinedSourceWave()
}

func (s *networkState) retainSourceServer() error {
	if s.current == nil {
		return errors.New("direct Source server has no current identity")
	}
	roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock, Create: true})
	if err != nil {
		return err
	}
	return errors.Join(roles.Replace(sourceProducer("server", s.config.root), []duty.Duty{sourceServerDuty(*s.current)}), roles.Close())
}

func sourceServerDuty(decision epoch.Decision) duty.Duty {
	return duty.Duty{Identity: decision.Snapshot.NodeID, Family: sha256.Sum256([]byte(decision.Snapshot.DeclaredFamily)),
		Class: "direct-source", State: "live", NotAfter: decision.Snapshot.ValidUntil}
}

// servingDutySet keeps distinct still-servable identity/family pairs. When a
// successor reuses both, its authenticated bound replaces the old record.
func servingDutySet(decision epoch.Decision, predecessors []duty.Duty) []duty.Duty {
	duties := append([]duty.Duty(nil), predecessors...)
	current := sourceServerDuty(decision)
	for index := range duties {
		if duties[index].Identity == current.Identity && duties[index].Family == current.Family {
			duties[index] = current
			return duties
		}
	}
	return append(duties, current)
}

func (s *networkState) releaseSourceServer() error {
	if !s.config.sourceInfo.Serving {
		return nil
	}
	roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
	if err != nil {
		return err
	}
	return errors.Join(roles.Remove(sourceProducer("server", s.config.root)), roles.Close())
}

func sourceProducer(kind, root string) [32]byte {
	return sha256.Sum256([]byte("ardents-h3-local-source-" + kind + "-v1\x00" + root))
}
