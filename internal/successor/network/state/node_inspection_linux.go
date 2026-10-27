package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// NodeHistory is copied public binding data from one checked retained signed
// generation. It carries no runtime observation, lease, producer or permission.
// An operation rechecks the original NodeInspection at its owning boundaries.
type NodeHistory struct {
	NetworkID, ProfileDigest, NodeID, PublicKey [32]byte
	DutyGeneration                              uint64
}

// NodeInspection independently retains existing signed State and stopped local
// participation. It creates, repairs, activates and prunes nothing, opens no
// runtime/Source and lends no CurrentRuntime or mutable storage handle.
type NodeInspection struct{ original *nodeInspection }

type nodeInspection struct {
	mu, ioMu           sync.Mutex
	users              sync.WaitGroup
	once               sync.Once
	parent, ctx        context.Context
	cancel             context.CancelFunc
	config             config
	state              *durable.Inspection
	roles              duty.NodeInspection
	node               [32]byte
	uid, gid           uint32
	closed             bool
	terminal, closeErr error
}

var errNodeInspectionClosed = errors.New("retained Node inspection closed")

// OpenNodeInspection binds the exact original configuration, numeric principal
// and Node. No missing history selects a fresh opening, and no expired producer
// can be removed in order to inspect a supposedly stopped installation.
func OpenNodeInspection(parent context.Context, input Config, node [32]byte, uid, gid uint32) (result NodeInspection, returned error) {
	if parent == nil || os.Geteuid() != 0 || node == [32]byte{} || !filepath.IsAbs(input.Root) || filepath.Clean(input.Root) != input.Root || !filepath.IsAbs(input.LocalRoleStateRoot) || filepath.Clean(input.LocalRoleStateRoot) != input.LocalRoleStateRoot || !filepath.IsAbs(input.ClockObservationFile) || filepath.Clean(input.ClockObservationFile) != input.ClockObservationFile || input.ObserveClock != nil || input.PermitWork != nil {
		return result, errors.New("invalid retained Node inspection")
	}
	if err := parent.Err(); err != nil {
		return result, err
	}
	if _, finite := parent.Deadline(); !finite {
		return result, errors.New("Node inspection requires finite original caller")
	}
	cfg, err := validateConfig(input)
	if err != nil {
		return result, err
	}
	if cfg.sourceInfo.Configured || cfg.sourceInfo.Serving || cfg.automatic != 0 {
		return result, errors.New("Node inspection is retained-history only")
	}
	ctx, cancel := context.WithCancel(parent)
	s := &nodeInspection{parent: parent, ctx: ctx, cancel: cancel, config: cfg, node: node, uid: uid, gid: gid}
	defer func() {
		if returned != nil {
			s.terminal = errors.Join(s.terminal, returned)
			returned = errors.Join(returned, s.close())
			result = NodeInspection{}
		}
	}()
	s.state, err = durable.OpenInspection(ctx, cfg.root, storageLimits())
	if err != nil {
		return result, err
	}
	if actualUID, actualGID := s.state.Principal(); actualUID != uid || actualGID != gid {
		return result, errors.New("retained Node State principal differs")
	}
	s.roles, err = duty.OpenNodeInspection(ctx, cfg.localRoles, uid, gid)
	if err != nil {
		return result, err
	}
	if _, err := s.inspect(); err != nil {
		return result, err
	}
	return NodeInspection{original: s}, nil
}

func (s *nodeInspection) inspect() (NodeHistory, error) {
	if err := s.ctx.Err(); err != nil {
		return NodeHistory{}, err
	}
	if err := s.state.Check(); err != nil {
		return NodeHistory{}, err
	}
	if err := s.roles.Check(); err != nil {
		return NodeHistory{}, err
	}
	current, journal, err := readRetainedInspection(s.config, s.state)
	if err != nil {
		return NodeHistory{}, err
	}
	generation, err := closedProfileGeneration(current.Snapshot.Generation)
	if err != nil {
		return NodeHistory{}, err
	}
	stored, raw, err := s.state.ReadClosedProfile(generation)
	if err != nil || stored == (durable.ClosedProfileState{}) || stored.Conflict != [32]byte{} || stored.Epoch != current.Snapshot.Epoch {
		return NodeHistory{}, errors.Join(errors.New("retained Node closed profile unavailable"), err)
	}
	now, err := trustedNow(s.config, journal)
	if err != nil {
		return NodeHistory{}, err
	}
	profile, err := closedprofile.Verify(raw, closedprofile.Context{StateGeneration: generation, NetworkID: current.Snapshot.NetworkID,
		EpochDigest: current.Snapshot.Digest, Epoch: current.Snapshot.Epoch, Authority: s.config.closedProfileAuthority, Now: now})
	if err != nil || profile.Digest != stored.Accepted {
		return NodeHistory{}, errors.Join(errors.New("retained Node profile authentication differs"), err)
	}
	bound, err := bindAcceptedMembership(current, profile)
	if err != nil {
		return NodeHistory{}, err
	}
	if err := s.state.Check(); err != nil {
		return NodeHistory{}, err
	}
	if err := s.roles.Check(); err != nil {
		return NodeHistory{}, err
	}
	// Fresh trusted time follows every native read and profile verification.
	// The temporary domain observation is never handed to an accepting consumer.
	observed, err := trustedObservation(s.config, journal)
	if err != nil {
		return NodeHistory{}, err
	}
	view, err := bound.accepted.Observe(observed)
	if err != nil {
		return NodeHistory{}, err
	}
	member, err := view.Member(s.node, observed.Instant())
	if err != nil {
		return NodeHistory{}, err
	}
	return NodeHistory{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, NodeID: member.NodeID,
		PublicKey: member.PublicKey, DutyGeneration: member.DutyGeneration}, s.ctx.Err()
}

// Check returns only copied original binding facts after full signed/time/native
// reobservation. Close denial and real failure prevent any later successful copy.
func (inspection NodeInspection) Check() (facts NodeHistory, returned error) {
	s := inspection.original
	if s == nil {
		return facts, errNodeInspectionClosed
	}
	s.mu.Lock()
	if s.closed || s.terminal != nil || s.ctx.Err() != nil {
		returned = errors.Join(errNodeInspectionClosed, s.terminal, s.parent.Err(), s.ctx.Err())
		s.mu.Unlock()
		return facts, returned
	}
	s.users.Add(1)
	s.mu.Unlock()
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	facts, returned = s.inspect()
	s.mu.Lock()
	defer s.mu.Unlock()
	if returned == nil && (s.closed || s.ctx.Err() != nil) {
		returned = errors.Join(errNodeInspectionClosed, s.parent.Err(), s.ctx.Err())
	}
	if returned != nil {
		facts = NodeHistory{}
		if !(s.closed && s.parent.Err() == nil && nodeInspectionLocalClosure(returned)) {
			s.terminal = errors.Join(s.terminal, returned)
		}
	}
	return facts, returned
}

func (inspection NodeInspection) Close() error {
	if inspection.original == nil {
		return nil
	}
	return inspection.original.close()
}

func (s *nodeInspection) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.cancel()
		s.mu.Unlock()
		s.users.Wait()
		physical := s.roles.Close()
		if s.state != nil {
			physical = errors.Join(physical, s.state.Close())
		}
		if s.parent.Err() == nil && nodeInspectionLocalClosure(physical) {
			physical = nil
		}
		s.closeErr = errors.Join(s.terminal, physical, s.parent.Err())
	})
	return s.closeErr
}

func nodeInspectionLocalClosure(err error) bool {
	if err == nil {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, leaf := range many.Unwrap() {
			if !nodeInspectionLocalClosure(leaf) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return nodeInspectionLocalClosure(one.Unwrap())
	}
	return err == context.Canceled || err == errNodeInspectionClosed
}
