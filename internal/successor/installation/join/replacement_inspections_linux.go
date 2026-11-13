package join

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Each history owner acquires its own existing-only lease. Installation receives
// no mutable ledger, producer, Budget or runtime observation from these handles.
type replacementInspections struct {
	state    state.NodeInspection
	spend    spending.Inspection
	hosting  hosting.Inspection
	expected state.NodeHistory
	closed   bool
	terminal error
}

func inspectReplacement(ctx context.Context, c *completedInitial) (result *replacementInspections, returned error) {
	return inspectStoppedHistories(ctx, c)
}

func inspectStoppedHistories(ctx context.Context, c *completedInitial) (result *replacementInspections, returned error) {
	s := &replacementInspections{}
	defer func() {
		if returned != nil {
			s.terminal = returned
			returned = errors.Join(returned, s.close())
			result = nil
		}
	}()
	p, err := DecodePlan(c.files[filepath.Join(c.sealed, "join-plan.json")].body)
	if err != nil {
		return nil, err
	}
	b := c.binding
	n := p.Network
	pins := make(map[[32]byte]ed25519.PublicKey, len(n.Authorities))
	for _, public := range n.Authorities {
		id := sha256.Sum256(public[:])
		if _, exists := pins[id]; exists || public == [32]byte{} {
			return nil, ErrBinding
		}
		pins[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	cfg := state.Config{Root: n.Root, NetworkID: n.NetworkID, Authorities: pins, Threshold: n.Threshold,
		ClosedProfileAuthority: append(ed25519.PublicKey(nil), n.ProfileAuthority[:]...), AcceptedProfile: "ardents-route-v3",
		Clock: time.Now, ClockObservationFile: n.ClockObservationFile, LocalRoleStateRoot: p.ParticipationRoot}
	s.state, err = state.OpenNodeInspection(ctx, cfg, p.NodeID, b.UID, b.GID)
	if err != nil {
		return nil, err
	}
	s.expected, err = s.state.Check()
	if err != nil {
		return nil, err
	}
	certificate := c.files[filepath.Join(b.Request.CredentialDirectory, "certificate.pem")]
	if certificate == nil {
		return nil, ErrBinding
	}
	block, rest := pem.Decode(certificate.body)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrBinding
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(key, s.expected.PublicKey[:]) || s.expected.NodeID != p.NodeID || s.expected.NetworkID != n.NetworkID {
		return nil, ErrBinding
	}
	s.spend, err = spending.OpenInspection(ctx, p.SpendRoot, spending.Binding{NetworkID: s.expected.NetworkID, ProfileDigest: s.expected.ProfileDigest, ReceiverNodeID: s.expected.NodeID, ReceiverDutyGeneration: s.expected.DutyGeneration}, b.UID, b.GID)
	if err != nil {
		return nil, err
	}
	s.hosting, err = hosting.OpenInspection(ctx, p.HostingRoot, string(c.files[filepath.Join(b.Request.HostingRoot, "budget.pin")].body), b.UID, b.GID)
	if err != nil {
		return nil, err
	}
	return s, s.check()
}

func (s *replacementInspections) check() error {
	if s == nil || s.closed {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	facts, err := s.state.Check()
	if err == nil && facts != s.expected {
		err = ErrBinding
	}
	if err == nil {
		err = s.spend.Check()
	}
	if err == nil {
		err = s.hosting.Check()
	}
	if err == nil {
		facts, err = s.state.Check()
		if err == nil && facts != s.expected {
			err = ErrBinding
		}
	}
	if err != nil {
		s.terminal = err
	}
	return err
}

func (s *replacementInspections) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		// State/participation remains retained until both physical accounting
		// owners have actually joined; no result or late pass erases failures.
		s.terminal = errors.Join(s.terminal, s.hosting.Close(), s.spend.Close())

		s.terminal = errors.Join(s.terminal, s.state.Close())
	}
	return s.terminal
}
