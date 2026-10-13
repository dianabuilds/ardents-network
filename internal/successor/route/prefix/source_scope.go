package prefix

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// CheckSourceCaller binds scope to the SAME original physical opening caller.
// A descendant attempt context, copied limits or another operation cannot
// acquire ownership of this Source. Route interprets no Job or local Grant.
func (p *Prefix) CheckSourceCaller(caller context.Context, networkID, profile [32]byte, end time.Time) error {
	if p == nil || p.self != p || caller == nil || !reflect.TypeOf(caller).Comparable() || p.caller != caller {
		return errors.New("original Source opening caller differs")
	}
	return p.CheckSourceScope(networkID, profile, end)
}

// CheckSourceScope compares caller facts with this genuine original Source.
// It reobserves authority; it never extends the physical/profile lifetime or
// accepts a role-3/4 prefix as a Source because copied fields match.
func (p *Prefix) CheckSourceScope(networkID, profile [32]byte, end time.Time) error {
	if p == nil || p.self != p || p.bootstrap || p.config.Leg.EntryMember.RoleDomain != 1 || networkID == [32]byte{} || profile == [32]byte{} || end.IsZero() || !time.Now().Before(end) || end != end.UTC().Truncate(time.Second) {
		return errors.New("original Source scope is unavailable")
	}
	if p.config.Leg.Profile.Network != networkID || p.config.Leg.Profile.Digest != profile || end.After(p.config.Deadline) || end.After(p.config.Leg.NotAfter) || end.After(p.config.Leg.Profile.NotAfter) {
		return errors.New("original Source scope differs")
	}
	return p.Current()
}

// CheckResponderScope observes this Source's own sealed role-3 child. It checks
// the physical parent and bounds only; recipient eligibility still belongs to
// CheckResponderRendezvous, and no Service or recovery authority follows.
func (p *Prefix) CheckResponderScope(responder *Prefix, end time.Time) error {
	if p == nil || p.self != p || responder == nil || responder.self != responder || responder.source != p ||
		p.config.Leg.EntryMember.RoleDomain != 1 || responder.config.Leg.EntryMember.RoleDomain != 3 || end.IsZero() || !time.Now().Before(end) ||
		end.After(responder.config.Deadline) || end.After(responder.config.Leg.NotAfter) || end.After(responder.config.Leg.Profile.NotAfter) {
		return errors.New("original Source-bound Responder scope differs")
	}
	return errors.Join(p.Current(), responder.Current())
}

// ResolveSubmission resolves only the Descriptor's signed Introduction Node
// through this Source's current authenticated membership and delivery policy.
// It chooses no substitute recipient or new physical Source.
func (p *Prefix) ResolveSubmission(node [32]byte, excluded []route.Member) (network.RetainedDuty, error) {
	if p == nil || p.self != p || p.bootstrap || p.config.Leg.EntryMember.RoleDomain != 1 || node == [32]byte{} {
		return network.RetainedDuty{}, errors.New("original submission Source is unavailable")
	}
	if err := p.localCurrent(); err != nil {
		return network.RetainedDuty{}, err
	}
	view, err := p.observeOriginal()
	if err != nil {
		return network.RetainedDuty{}, err
	}
	// The original leg and recipient share this fresh observation. No I/O
	// separates their checks; the final Current below independently guards
	// the result after resolution without retaining an accepting cache.
	if err := errors.Join(p.config.Leg.Check(view, view.ObservedAt()), p.localCurrent()); err != nil {
		return network.RetainedDuty{}, err
	}
	duty, err := view.RetainDuty(node, view.ObservedAt())
	if err != nil {
		return network.RetainedDuty{}, err
	}
	exact, err := p.config.Leg.SubmissionDuty(view, node, duty.RecordGeneration, excluded)
	if err != nil || exact != duty {
		return network.RetainedDuty{}, errors.Join(errors.New("original submission recipient differs"), err)
	}
	return duty, p.Current()
}
