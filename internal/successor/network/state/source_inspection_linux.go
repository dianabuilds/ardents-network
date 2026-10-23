package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

var errSourceInspectionClosed = errors.New("retained Source inspection closed")

// SourceInspection owns a bounded read-only semantic observation of retained
// Source State and participation. It lends no root/lease, initializes/repairs
// nothing, constructs no runtime/listener and returns no CurrentRuntime right.
// Copies retain the same original caller, denial and physical completion.
type SourceInspection struct{ original *sourceInspection }
type sourceInspection struct {
	mu          sync.Mutex // local admission/result only, never foreign I/O or join
	ioMu        sync.Mutex
	uses        sync.WaitGroup
	parent, ctx context.Context
	cancel      context.CancelFunc
	config      config
	clockPath   string
	state       *durable.SourceInspection
	roles       *duty.SourceInspection
	uid, gid    uint32
	closed      bool
	terminal    error
	closeOnce   sync.Once
	closeErr    error
}

func OpenSourceInspection(parent context.Context, input Config) (result SourceInspection, err error) {
	if parent == nil || parent.Err() != nil || os.Geteuid() != 0 {
		return result, errors.New("invalid retained Source inspection caller")
	}
	if _, bounded := parent.Deadline(); !bounded {
		return result, errors.New("Source inspection requires a finite original caller")
	}
	resolved, err := validateConfig(input)
	if err != nil {
		return result, err
	}
	if resolved.sourceInfo.Configured || resolved.sourceInfo.Serving || resolved.automatic != 0 || resolved.localRoles == "" || input.ClockObservationFile == "" || input.ObserveClock != nil {
		return result, errors.New("Source inspection is retained-history only")
	}
	ctx, cancel := context.WithCancel(parent)
	s := &sourceInspection{parent: parent, ctx: ctx, cancel: cancel, config: resolved, clockPath: input.ClockObservationFile}
	defer func() {
		if err != nil {
			s.terminal = errors.Join(s.terminal, err)
			err = errors.Join(err, s.close())
			result = SourceInspection{}
		}
	}()
	s.state, err = durable.OpenSourceInspection(ctx, resolved.root, storageLimits())
	if err != nil {
		return result, err
	}
	s.uid, s.gid = s.state.Principal()
	s.roles, err = duty.OpenSourceInspection(ctx, resolved.localRoles, s.uid, s.gid)
	if err != nil {
		return result, err
	}
	if err := s.inspect(); err != nil {
		return result, err
	}
	return SourceInspection{original: s}, nil
}

// Check binds the State-owned policy and original native principal to the exact
// Source declaration consumed by Installation. Detached configuration/facts,
// another caller or already closed copies cannot recreate this observation.
func (inspection SourceInspection) Check(ctx context.Context, raw []byte, uid, gid uint32) (err error) {
	s := inspection.original
	if s == nil || ctx == nil || ctx != s.parent {
		return errSourceInspectionClosed
	}
	s.mu.Lock()
	if s.closed || s.terminal != nil || s.parent.Err() != nil {
		err = errors.Join(errSourceInspectionClosed, s.terminal, s.parent.Err())
		s.mu.Unlock()
		return err
	}
	s.uses.Add(1)
	s.mu.Unlock()
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if err = s.policy(raw, uid, gid); err == nil {
		err = s.inspect()
	}
	s.mu.Lock()
	if err == nil && (s.closed || s.ctx.Err() != nil) {
		err = errors.Join(errSourceInspectionClosed, s.parent.Err(), s.ctx.Err())
	}
	if err != nil && !(s.closed && s.parent.Err() == nil && sourceInspectionLocalClosure(err)) {
		s.terminal = errors.Join(s.terminal, err)
	}
	s.mu.Unlock()
	return err
}

func (s *sourceInspection) policy(raw []byte, uid, gid uint32) error {
	plan, err := source.DecodeServingDeclaration(raw)
	if err != nil {
		return err
	}
	n := plan.Network
	if n == nil || uid != s.uid || gid != s.gid || n.Root != s.config.root || plan.ParticipationRoot != s.config.localRoles || n.ClockObservationFile != s.clockPath || n.NetworkID != s.config.networkID || n.Threshold != s.config.threshold ||
		!bytes.Equal(n.ProfileAuthority[:], s.config.closedProfileAuthority) || s.config.acceptedProfile != "ardents-route-v3" || len(n.Authorities) != len(s.config.authorities) {
		return errors.New("retained Source inspection policy differs")
	}
	seen := map[[32]byte]bool{}
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if seen[id] || !bytes.Equal(s.config.authorities[id], public[:]) {
			return errors.New("retained Source authority differs")
		}
		seen[id] = true
	}
	return nil
}

func (s *sourceInspection) inspect() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.state.Check(); err != nil {
		return err
	}
	if err := s.roles.Check(); err != nil {
		return err
	}
	name, values, err := s.state.ReadState()
	if err != nil {
		return err
	}
	current, err := loadCurrentValues(s.config, name, values)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrNoCurrentGeneration
	}
	control, raw, err := s.state.ReadControl()
	if err != nil {
		return err
	}
	journal, err := decodeDistributionState(raw)
	if err != nil || control == "" || distributionDigest(raw) != control || journal.epochFloor != current.Header.Number || journal.epochDigest != current.Header.Digest || journal.conflicting || journal.cycleActive {
		return errors.Join(errors.New("retained Source security history unavailable without explicit recovery"), err)
	}
	if journal.pendingDigest != [32]byte{} {
		if journal.pendingDigest == current.Header.Digest {
			return errors.New("retained Source pending state requires explicit recovery")
		}
		found := false
		for _, value := range values {
			if value.Name != fmt.Sprintf("%x", journal.pendingDigest) {
				continue
			}
			decision, err := loadGeneration(s.config, value, epochPredecessor(current))
			if err != nil || decision.Header.ValidFrom.Unix() != journal.pendingValidFrom {
				return errors.Join(errors.New("retained Source pending history differs"), err)
			}
			if err := classifyRetainedClosedSchema(s.config, decision.Header, "inspection pending"); err != nil {
				return err
			}
			found = true
		}
		if !found {
			return errors.New("retained Source pending generation missing")
		}
	}
	if err := s.state.Check(); err != nil {
		return err
	}
	if err := s.roles.Check(); err != nil {
		return err
	}
	// Trusted time is sampled after original physical reads. No wall-clock
	// fallback, pending activation, watermark reset or repair is performed.
	now, err := trustedNow(s.config, journal)
	if err != nil {
		return err
	}
	if now.Before(current.Header.ValidFrom) || !now.Before(current.Snapshot.ValidUntil) {
		return errors.New("retained Source State is not current")
	}
	return s.ctx.Err()
}

func (inspection SourceInspection) Close() error {
	if inspection.original == nil {
		return nil
	}
	return inspection.original.close()
}
func (s *sourceInspection) close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.cancel()
		s.mu.Unlock()
		s.uses.Wait()
		var physical error
		if s.roles != nil {
			physical = errors.Join(physical, s.roles.Close())
		}
		if s.state != nil {
			physical = errors.Join(physical, s.state.Close())
		}
		if s.parent.Err() == nil && sourceInspectionLocalClosure(physical) {
			physical = nil
		}
		s.mu.Lock()
		s.closeErr = errors.Join(s.terminal, physical, s.parent.Err())
		s.mu.Unlock()
	})
	return s.closeErr
}

func sourceInspectionLocalClosure(err error) bool {
	if err == nil {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, leaf := range many.Unwrap() {
			if !sourceInspectionLocalClosure(leaf) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return sourceInspectionLocalClosure(one.Unwrap())
	}
	return err == context.Canceled || err == errSourceInspectionClosed
}
