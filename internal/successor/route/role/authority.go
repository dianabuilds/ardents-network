package role

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Authority retains an exact duty from one coherent observation. Current is
// invoked outside transport locks; an observation never becomes a live lease.
type Authority struct {
	Current func() (network.RuntimeView, error)
	Duty    network.RetainedDuty
	Profile network.ProfileBinding
}

// FreshHello creates a fresh channel nonce while preserving the requested
// original recipient and bounds. Reobservation after nonce creation prevents
// publishing a HELLO after authority loss; callers still check around I/O.
func (a Authority) FreshHello(end time.Time, purpose ardp.Purpose, outer bool) (ardp.Hello, error) {
	m, err := a.Member()
	if err != nil {
		return ardp.Hello{}, err
	}
	p := a.Profile
	h := ardp.Hello{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest,
		RecipientNodeID: m.NodeID, RecipientDutyGeneration: m.DutyGeneration, Purpose: purpose, Deadline: end}
	if _, err := rand.Read(h.ChannelNonce[:]); err != nil {
		return ardp.Hello{}, err
	}
	if _, err := a.Hello(h, outer); err != nil {
		return ardp.Hello{}, err
	}
	return h, nil
}

// Member reobserves the original exact duty and profile; it never rebinds them.
func (a Authority) Member() (network.Member, error) {
	if a.Current == nil {
		return network.Member{}, errors.New("route observer absent")
	}
	v, err := a.Current()
	if err != nil {
		return network.Member{}, err
	}
	if v.Profile().ProfileBinding != a.Profile {
		return network.Member{}, errors.New("route profile changed")
	}
	if err = v.MatchDuty(a.Duty, time.Now()); err != nil {
		return network.Member{}, err
	}
	return v.Member(a.Duty.NodeID, time.Now())
}

// Hello checks this channel against the original duty, purpose and bounds.
func (a Authority) Hello(h ardp.Hello, outer bool) (network.Member, error) {
	m, err := a.Member()
	if err != nil {
		return m, err
	}
	p := a.Profile
	if h.NetworkID != p.Network || h.StateGeneration != p.Generation || h.StateDigest != p.EpochDigest ||
		h.ProfileDigest != p.Digest || h.RecipientNodeID != m.NodeID || h.RecipientDutyGeneration != m.DutyGeneration ||
		!time.Now().Before(h.Deadline) || h.Deadline.After(m.NotAfter()) || h.Deadline.After(p.NotAfter) ||
		(outer && h.Purpose != ardp.PurposeForwarding) || (!outer && !route.PurposePermitsDuty(uint8(h.Purpose), m.RoleDomain, m.Subrole)) {
		return network.Member{}, errors.New("route HELLO binding unavailable")
	}
	return m, nil
}

// Conflicting applies Route's participant exclusion to Network member facts.
func Conflicting(a, b network.Member) bool {
	return route.Conflict(route.Member{NodeID: a.NodeID, PublicKey: a.PublicKey, FamilyID: a.FamilyID}, route.Member{NodeID: b.NodeID, PublicKey: b.PublicKey, FamilyID: b.FamilyID})
}
