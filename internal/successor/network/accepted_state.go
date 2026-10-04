package network

import (
	"errors"
	"time"
)

// ProfileBinding identifies the authenticated role statement within one
// accepted generation. Issuer inventory remains copied public profile data.
type ProfileBinding struct {
	Network, Generation, EpochDigest, Digest [32]byte
	Epoch                                    uint64
	NotBefore, NotAfter                      time.Time
}

// AcceptedState is the coherent domain value reconstructed after acceptance
// or recovery. It has no transport, storage, goroutine or process-resource owner.
// Only a trusted observation of this value can produce a RuntimeView.
type AcceptedState struct {
	epoch      EpochFacts
	profile    ProfileFacts
	membership Membership
}

func BindAcceptedState(epoch EpochFacts, generation [32]byte, profile ProfileFacts, membership Membership) (AcceptedState, error) {
	if !epoch.valid() || generation == [32]byte{} || profile.Digest == [32]byte{} || profile.Network != epoch.Network ||
		profile.Generation != generation || profile.EpochDigest != epoch.Digest || profile.Epoch != epoch.Number ||
		profile.NotBefore.Before(epoch.ValidFrom) || profile.NotAfter.After(epoch.ValidUntil) || !profile.NotAfter.After(profile.NotBefore) || len(membership.members) == 0 || membership.profile != profile.ProfileBinding || !profile.matchesIssuer(membership) {
		return AcceptedState{}, errors.New("profile and membership do not belong to accepted network generation")
	}
	return AcceptedState{epoch: epoch, profile: profile.copied(), membership: membership}, nil
}

// RuntimeView is an immutable, bounded observation, never an operation lease.
// Consumers must obtain a new observation at their existing admission points.
type RuntimeView struct {
	accepted   AcceptedState
	observed   TrustedTime
	membership Membership
}

func (accepted AcceptedState) Observe(observed TrustedTime) (RuntimeView, error) {
	view := RuntimeView{accepted: accepted, observed: observed, membership: accepted.membership.Observe(observed.Instant())}
	if err := view.Check(observed.Instant()); err != nil {
		return RuntimeView{}, err
	}
	return view, nil
}

func (view RuntimeView) ObservedAt() time.Time { return view.observed.Instant() }

// Profile returns copied public facts from the same accepted generation as
// members and trusted time. The signed interval is not shortened to any one
// member's operation horizon, and issuer reachability does not grant authority.
func (view RuntimeView) Profile() ProfileFacts { return view.accepted.profile.copied() }

func (view RuntimeView) Check(now time.Time) error {
	observed := view.ObservedAt()
	if observed.IsZero() || now.IsZero() || !view.accepted.epoch.valid() {
		return errors.New("network observation is unavailable")
	}
	if now.Before(observed) {
		now = observed
	}
	profile := view.accepted.profile
	if now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) || !now.Before(view.accepted.epoch.ValidUntil) {
		return errors.New("network observation is no longer current")
	}
	return nil
}

func (view RuntimeView) Members() []Member { return view.membership.Members() }

func (view RuntimeView) Member(id [32]byte, now time.Time) (Member, error) {
	if err := view.Check(now); err != nil {
		return Member{}, err
	}
	return view.membership.Member(id, now)
}

func (view RuntimeView) MemberByKey(key [32]byte, now time.Time) (Member, error) {
	if err := view.Check(now); err != nil {
		return Member{}, err
	}
	return view.membership.MemberByKey(key, now)
}
