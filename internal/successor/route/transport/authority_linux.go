//go:build linux

package transport

import (
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

func (a Authority) member() (network.Member, error) {
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

func (a Authority) hello(h ardp.Hello, outer bool) (network.Member, error) {
	m, err := a.member()
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

func conflicting(a, b network.Member) bool {
	return a.NodeID == b.NodeID || a.PublicKey == b.PublicKey || a.FamilyID == b.FamilyID
}
