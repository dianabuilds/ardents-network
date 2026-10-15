package prefix

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// Detached model facts isolate same-leg supervision scheduling. They supply
// neither signed Network authority nor successful transport or Service work.
func TestJoinRecipientCompletionDefersOnlySameLegSupervision(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		binding := network.ProfileBinding{Network: [32]byte{101}, Generation: [32]byte{102}, EpochDigest: [32]byte{103}, Digest: [32]byte{104}, Epoch: 1, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
		profile := network.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: [32]byte{105}, IssuerNodeID: [32]byte{4}, IssuerDutyGeneration: 1, TokenKeys: []network.TokenKey{{WindowStart: now.Truncate(time.Hour), Class: 2, SPKI: [346]byte{1}}}}
		var records []network.NodeRecord
		var assignments []network.Assignment
		for id := byte(1); id <= 4; id++ {
			domain, subrole, name := uint8(1), id, "initiator"
			if id >= 3 {
				domain, subrole, name = 2, 4, "rendezvous"
			}
			if id == 4 {
				subrole = 6
			}
			record := network.NodeRecord{NodeID: [32]byte{id}, RecordDigest: [32]byte{id + 60}, PublicKey: [32]byte{id + 20}, FamilyID: [32]byte{id + 40}, Generation: 1, Assignment: name, CarrierProfile: "ardents-carrier-tcp-tls-v2", Endpoint: "127.0.0.1:2000", Capacity: 1, ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter, AssignmentNotAfter: binding.NotAfter}
			records = append(records, record)
			assignments = append(assignments, network.Assignment{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: 1, RoleDomain: domain, Subrole: subrole})
		}
		members, err := network.BindMembership(binding, assignments, records)
		if err != nil {
			t.Fatal(err)
		}
		accepted, err := network.BindAcceptedState(network.EpochFacts{Network: binding.Network, Number: binding.Epoch, Digest: binding.EpochDigest, ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter}, binding.Generation, profile, members)
		if err != nil {
			t.Fatal(err)
		}
		clock, err := network.ConfirmTime(network.ClockEvidence{Wall: now, Independent: now}, 0)
		if err != nil {
			t.Fatal(err)
		}
		view, err := accepted.Observe(clock)
		if err != nil {
			t.Fatal(err)
		}
		entry, _ := view.RetainDuty([32]byte{1}, now)
		interior, _ := view.RetainDuty([32]byte{2}, now)
		incoming, _ := view.RetainDuty([32]byte{3}, now)
		em, _ := view.Member(entry.NodeID, now)
		im, _ := view.Member(interior.NodeID, now)
		caller, cancel := context.WithCancel(t.Context())
		defer cancel()
		calls := 0
		p := &Prefix{caller: caller, ctx: caller, observation: &prefixObservation{at: now.Add(-time.Second)}, config: Config{Deadline: now.Add(time.Minute),
			Leg:     selection.Leg{Entry: entry, Interior: interior, EntryMember: em, InteriorMember: im, Profile: binding, NotAfter: binding.NotAfter},
			Current: func() (network.RuntimeView, error) { calls++; return view, nil }}}
		for range 2 {
			time.Sleep(time.Millisecond)
			if err := p.joinRecipient(incoming, p.config.Deadline); err != nil || !p.observation.at.Equal(time.Now()) {
				t.Fatal("complete same-leg recipient observation did not defer supervision", err)
			}
		}
		if calls != 2 {
			t.Fatal("recipient effects reused accepting authority", calls)
		}
		last := p.observation.at
		foreign := incoming
		foreign.RecordGeneration++
		if err := p.joinRecipient(foreign, p.config.Deadline); err == nil || !p.observation.at.Equal(last) {
			t.Fatal("foreign recipient deferred supervision", err)
		}
		p.config.Current = func() (network.RuntimeView, error) { calls++; cancel(); return view, nil }
		if err := p.joinRecipient(incoming, p.config.Deadline); !errors.Is(err, context.Canceled) || !p.observation.at.Equal(last) {
			t.Fatal("revoked original caller deferred supervision", err)
		}
	})
}

func TestResolveRendezvousRequiresOriginalSourceBeforeObservation(t *testing.T) {
	observations := 0
	detached := &Prefix{config: Config{Current: func() (network.RuntimeView, error) {
		observations++
		return network.RuntimeView{}, nil
	}}}
	for _, source := range []*Prefix{nil, detached} {
		duty, end, err := source.ResolveRendezvous([32]byte{1}, 1, nil)
		if err == nil || duty != (network.RetainedDuty{}) || end != (time.Time{}) {
			t.Fatal("detached Source supplied independently checked recipient")
		}
	}
	if observations != 0 {
		t.Fatal("detached role reached Network effects")
	}
}
