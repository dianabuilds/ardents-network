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
