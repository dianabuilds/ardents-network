//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func addTextIntroductionPrefixState(source *textSourceStateFixture) {
	source.view.NodeCount, source.snapshot.CandidateCount = 11, 11
	for index := 7; index < 11; index++ {
		subrole := uint8(1)
		if index >= 9 {
			subrole = 2
		}
		id, record := fixtureID(byte(10+index)), fixtureID(byte(30+index))
		source.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: id, RecordDigest: record, RoleDomain: 4, Subrole: subrole, DutyGeneration: uint64(index + 1)}
		candidate := source.snapshot.Candidates[4]
		candidate.NodeID, candidate.RecordDigest = id, record
		source.snapshot.Candidates[index] = candidate
	}
}

// Installed-worker and accepted-State facts remain explicit fixtures. Eleven
// actual Node runtimes, Custody issuance, journal and both separate prefixes
// are production paths; this does not claim command publication readiness.
func TestTextPublisherIntroductionPrefixUsesSeparateDomainAndRealIssuance(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, _ := startTextRoleNetwork(t, carrier, true, true)
			source, err := owner.openTextPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			introduction, err := owner.openTextIntroductionPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			separate := owner.prefix == source && owner.introduction.prefix == introduction && source != introduction && owner.sourceSet != owner.introduction.set && owner.sourceSet.interior[0].Domain == 1 && owner.introduction.set.interior[0].Domain == 4 && owner.permission.batches == 2 && owner.issuance == nil
			owner.mu.Unlock()
			if !separate {
				t.Fatal("Publisher Introduction reused Source ownership or bootstrap admission")
			}
			if _, err := owner.openTextIntroductionPrefix(t.Context()); err == nil {
				t.Fatal("second live Introduction prefix opened")
			}
			registered, err := owner.registerTextIntroduction(t.Context(), 1, time.Now().UTC().Add(60*time.Second).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if registered.node == [32]byte{} || registered.request.Slot == [32]byte{} {
				t.Fatal("registration lost its State recipient or random slot")
			}
			if err := owner.withdrawTextIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-registered.channel.Done():
			default:
				t.Fatal("withdrawal did not join registration")
			}
			second, err := owner.registerTextIntroduction(t.Context(), 2, time.Now().UTC().Add(60*time.Second).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if second.request.Slot == registered.request.Slot {
				t.Fatal("registration reused withdrawn slot")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-second.channel.Done():
			default:
				t.Fatal("context released a live registration")
			}
			for _, prefix := range []*route.ClosedSourcePrefix{source, introduction} {
				select {
				case <-prefix.Done():
				default:
					t.Fatal("context released a live role prefix")
				}
			}
		})
	}
}
