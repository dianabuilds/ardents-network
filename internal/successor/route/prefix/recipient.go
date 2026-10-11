package prefix

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// Rendezvous checks a context's retained choice against this original
// physical leg. Selection keeps the choices and observation floors; Prefix
// supplies its immutable leg and bounds, without exposing either to JOIN.
// The caller retains the original generation through observation and commits
// its own result with CommitPair after this potentially durable work returns.
func (p *Prefix) Rendezvous(choice *selection.Rendezvous, slot uint8, excluded []route.Member) (network.RetainedDuty, time.Time, error) {
	duty, err := choice.DutyForLeg(p.config.Leg, slot, excluded)
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	return duty, minDeadline(choice.NotAfter(), p.config.Deadline), nil
}

// CheckRendezvous verifies the exact received duty through this Prefix's
// original observer and leg. It never selects an alternative or renews a bound.
// Observations run outside the generation locks, as before pair publication.
func (p *Prefix) CheckRendezvous(incoming network.RetainedDuty, excluded []route.Member) error {
	view, err := p.observeOriginal()
	if err != nil {
		return err
	}
	duty, err := p.config.Leg.RendezvousDuty(view, incoming.NodeID, incoming.RecordGeneration, excluded)
	if err != nil || duty != incoming {
		return errors.Join(errors.New("route JOIN exact incoming recipient unavailable"), err)
	}
	return nil
}

// CheckResponderRendezvous observes both original physical legs and refuses a
// Responder belonging to any other Source. The recipient must be eligible from
// both legs; this check neither opens JOIN nor publishes a Connection binding.
func (p *Prefix) CheckResponderRendezvous(responder *Prefix, incoming network.RetainedDuty, excluded []route.Member) error {
	if p == nil || responder == nil || responder.source != p || p.config.Leg.EntryMember.RoleDomain != 1 || responder.config.Leg.EntryMember.RoleDomain != 3 {
		return errors.New("route Responder original Source mismatch")
	}
	return errors.Join(p.CheckRendezvous(incoming, excluded), responder.CheckRendezvous(incoming, excluded))
}

// ResolveRendezvous independently resolves capsule candidate identifiers through
// this original Source observation. The sole issuer and resolution duty are
// excluded by identity, key and known family in the same observation. Neither
// a copied duty nor a requester endpoint can supply an accepting recipient.
func (p *Prefix) ResolveRendezvous(node [32]byte, generation uint64, excluded []route.Member) (network.RetainedDuty, time.Time, error) {
	if p == nil || p.config.Leg.EntryMember.RoleDomain != 1 {
		return network.RetainedDuty{}, time.Time{}, errors.New("original Source recipient unavailable")
	}
	view, err := p.observeOriginal()
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	issuer, err := p.config.Leg.IssuerDuty(view)
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	resolution, err := p.config.Leg.ResolutionDuty(view, excluded)
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	known := append([]route.Member(nil), excluded...)
	for _, duty := range []network.RetainedDuty{issuer, resolution} {
		known = append(known, route.Member{NodeID: duty.NodeID, PublicKey: duty.PublicKey, FamilyID: duty.FamilyID})
	}
	duty, err := p.config.Leg.RendezvousDuty(view, node, generation, known)
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	end := minDeadline(p.config.Deadline, p.config.Leg.Profile.NotAfter)
	end = minDeadline(end, duty.Epoch.ValidUntil)
	end = minDeadline(end, duty.RecordValidUntil)
	return duty, end, nil
}

// joinRecipient keeps every original parent, recipient, Epoch and profile
// horizon at the frame-effect boundary. The acquisition checks its caller and
// each original lifetime on both sides of these potentially durable reads.
func (p *Prefix) joinRecipient(incoming network.RetainedDuty, end time.Time) error {
	view, err := p.observeOriginal()
	if err != nil {
		return err
	}
	duty, err := p.config.Leg.RendezvousDuty(view, incoming.NodeID, incoming.RecordGeneration, nil)
	if err != nil || duty != incoming || end.After(p.config.Deadline) || end.After(incoming.RecordValidUntil) || end.After(incoming.Epoch.ValidUntil) || end.After(p.config.Leg.Profile.NotAfter) || !time.Now().Before(end) {
		return errors.Join(errors.New("route JOIN recipient or horizon differs"), err)
	}
	return nil
}
