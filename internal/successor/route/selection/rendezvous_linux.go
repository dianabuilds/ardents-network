//go:build linux

package selection

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// Rendezvous retains public recipient choices in one local Route context.
// It owns no JOIN, secret, transport, admission or Service authority. Keep this
// same owner after failed Duty calls; failures and expiry never draw a new set.
type Rendezvous struct {
	mu       sync.Mutex
	leg      Leg
	current  func() (network.RuntimeView, error)
	excluded []route.Member
	duties   []network.RetainedDuty
	members  []network.Member
	last     time.Time
	until    time.Time
	failure  error
}

// NewRendezvous constructs an initially unselected local-context owner. Only
// the initiating Domain 1 Source selects; a Responder checks the exact received
// public choice with Leg.RendezvousDuty instead. No authority is granted here.
func NewRendezvous(leg Leg, current func() (network.RuntimeView, error), excluded []route.Member) (*Rendezvous, error) {
	if current == nil || leg.EntryMember.RoleDomain != 1 || leg.InteriorMember.RoleDomain != 1 {
		return nil, errors.New("Rendezvous Source selection unavailable")
	}
	leg.known = append([]route.Member(nil), leg.known...)
	r := &Rendezvous{leg: leg, current: current}
	r.remember(leg.known)
	r.remember([]route.Member{rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember)})
	r.remember(excluded)
	return r, nil
}

// NotAfter is the immutable retention bound, or zero before the first draw.
func (r *Rendezvous) NotAfter() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.until
}

// Duty returns only the requested retained slot (initial 0, optional alternative
// 1), after observing current Network authority and all supplied known peers.
// On the first call it fixes both choices before reobserving. Losing authority
// during that second observation does not discard or replace the choices.
func (r *Rendezvous) Duty(slot uint8, excluded []route.Member) (network.RetainedDuty, error) {
	return r.DutyForLeg(r.leg, slot, excluded)
}

// DutyForLeg checks the retained context choice against the current physical
// Source leg. Reopening a prefix never resets choices, validity, observation
// floors or known exclusions. A conflicting or unavailable leg refuses without
// replacing the original selection. Duty continues to use the constructor leg.
func (r *Rendezvous) DutyForLeg(leg Leg, slot uint8, excluded []route.Member) (network.RetainedDuty, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if slot > 1 {
		return network.RetainedDuty{}, errors.New("Rendezvous choice outside retained set")
	}
	if leg.EntryMember.RoleDomain != 1 || leg.InteriorMember.RoleDomain != 1 {
		return network.RetainedDuty{}, errors.New("Rendezvous Source leg unavailable")
	}
	if r.failure != nil {
		return network.RetainedDuty{}, r.failure
	}
	r.remember(leg.known)
	r.remember([]route.Member{rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember)})
	r.remember(excluded)
	view, err := r.observe(leg)
	if err != nil {
		return network.RetainedDuty{}, err
	}
	if len(r.duties) == 0 {
		var eligible []network.Member
		for _, candidate := range view.Members() {
			if _, err := leg.RendezvousDuty(view, candidate.NodeID, candidate.DutyGeneration, r.excluded); err == nil {
				eligible = append(eligible, candidate)
			}
		}
		selected, drawErr := chooseRendezvous(eligible, rand.Reader)
		if len(selected) == 0 {
			return network.RetainedDuty{}, drawErr
		}
		// Retain before any further observer call. No post-draw failure can
		// make a later call select an expanding set of peers.
		r.members = selected
		r.until = minTime(view.ObservedAt().Add(30*time.Minute), view.Profile().NotAfter)
		for _, member := range selected {
			duty, err := view.RetainDuty(member.NodeID, view.ObservedAt())
			if err != nil {
				r.failure = err
				return network.RetainedDuty{}, err
			}
			r.duties = append(r.duties, duty)
			r.until = minTime(r.until, member.NotAfter())
		}
		if drawErr != nil {
			r.failure = drawErr
			return network.RetainedDuty{}, drawErr
		}
		view, err = r.observe(leg)
		if err != nil {
			return network.RetainedDuty{}, err
		}
	}
	if int(slot) >= len(r.duties) || !view.ObservedAt().Before(r.until) {
		return network.RetainedDuty{}, errors.New("Rendezvous retained choice unavailable")
	}
	duty := r.duties[slot]
	if err := view.MatchDuty(duty, view.ObservedAt()); err != nil {
		return network.RetainedDuty{}, err
	}
	member, err := view.Member(duty.NodeID, view.ObservedAt())
	if err != nil || member.RecordDigest != r.members[slot].RecordDigest {
		return network.RetainedDuty{}, errors.Join(errors.New("Rendezvous retained Record changed"), err)
	}
	if _, err := leg.RendezvousDuty(view, duty.NodeID, duty.RecordGeneration, r.excluded); err != nil {
		return network.RetainedDuty{}, err
	}
	return duty, nil
}

func (r *Rendezvous) observe(leg Leg) (network.RuntimeView, error) {
	view, err := r.current()
	if err != nil {
		return network.RuntimeView{}, err
	}
	if view.ObservedAt().Before(r.last) {
		return network.RuntimeView{}, errors.New("Rendezvous Source observation unavailable")
	}
	r.last = view.ObservedAt()
	if view.Profile().ProfileBinding != r.leg.Profile || leg.Check(view, view.ObservedAt()) != nil {
		return network.RuntimeView{}, errors.New("Rendezvous Source observation unavailable")
	}
	return view, nil
}

// remember owns a monotonic copy of known identities. Repeated use of one leg
// cannot grow the history; changing a physical leg cannot forget earlier peers.
func (r *Rendezvous) remember(peers []route.Member) {
	for _, peer := range peers {
		identity := route.Member{NodeID: peer.NodeID, PublicKey: peer.PublicKey, FamilyID: peer.FamilyID}
		if identity == (route.Member{}) {
			continue
		}
		present := false
		for _, known := range r.excluded {
			if known == identity {
				present = true
				break
			}
		}
		if !present {
			r.excluded = append(r.excluded, identity)
		}
	}
}

// RendezvousDuty resolves an exact incoming Node/duty using a current coherent
// observation and this Source or Responder leg. It never draws or replaces a
// recipient and accepts no supplied endpoint. It is a Route eligibility check,
// not capsule authentication or publication readiness.
func (leg Leg) RendezvousDuty(view network.RuntimeView, node [32]byte, generation uint64, excluded []route.Member) (network.RetainedDuty, error) {
	if (leg.EntryMember.RoleDomain != 1 && leg.EntryMember.RoleDomain != 3) || leg.Check(view, view.ObservedAt()) != nil {
		return network.RetainedDuty{}, errors.New("Rendezvous prefix unavailable")
	}
	member, err := view.Member(node, view.ObservedAt())
	if err != nil || member.DutyGeneration != generation || !route.PurposePermitsDuty(6, member.RoleDomain, member.Subrole) {
		return network.RetainedDuty{}, errors.Join(errors.New("exact Rendezvous duty unavailable"), err)
	}
	byKey, err := view.MemberByKey(member.PublicKey, view.ObservedAt())
	if err != nil || byKey.NodeID != member.NodeID {
		return network.RetainedDuty{}, errors.Join(errors.New("Rendezvous key ambiguous"), err)
	}
	known := append(append([]route.Member(nil), leg.known...), excluded...)
	known = append(known, rendezvousPeer(leg.EntryMember), rendezvousPeer(leg.InteriorMember))
	for _, peer := range known {
		if route.Conflict(rendezvousPeer(member), peer) {
			return network.RetainedDuty{}, errors.New("Rendezvous conflicts with known participant")
		}
	}
	return view.RetainDuty(member.NodeID, view.ObservedAt())
}

func rendezvousPeer(member network.Member) route.Member {
	return route.Member{NodeID: member.NodeID, PublicKey: member.PublicKey, FamilyID: member.FamilyID}
}

func chooseRendezvous(eligible []network.Member, random io.Reader) ([]network.Member, error) {
	if len(eligible) == 0 {
		return nil, errors.New("eligible Rendezvous absent")
	}
	index, err := rand.Int(random, big.NewInt(int64(len(eligible))))
	if err != nil {
		return nil, err
	}
	first := eligible[index.Int64()]
	selected := []network.Member{first}
	var alternatives []network.Member
	for _, candidate := range eligible {
		if !route.Conflict(rendezvousPeer(first), rendezvousPeer(candidate)) {
			alternatives = append(alternatives, candidate)
		}
	}
	if len(alternatives) != 0 {
		index, err = rand.Int(random, big.NewInt(int64(len(alternatives))))
		if err != nil {
			return selected, err
		}
		selected = append(selected, alternatives[index.Int64()])
	}
	return selected, nil
}
