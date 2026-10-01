//go:build linux

package endpoint

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func addIntroductionPrefixState(source *sourceStateFixture) {
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
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, _ := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true})
			source, err := owner.openPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			introductionPrefix, err := owner.openIntroductionPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			separate := owner.source.CurrentLocked() == source && owner.introduction.prefix.currentLocked() == introductionPrefix && owner.sourceSet != owner.introduction.prefix.set && owner.sourceSet.interior[0].Domain == 1 && owner.introduction.prefix.set.interior[0].Domain == 4 && owner.tokens.Permission.Batches == 2 && owner.tokens.Issuance == nil
			owner.mu.Unlock()
			if !separate {
				t.Fatal("Publisher Introduction reused Source ownership or bootstrap admission")
			}
			if _, err := owner.openIntroductionPrefix(t.Context()); err == nil {
				t.Fatal("second live Introduction prefix opened")
			}
			registered, err := owner.registerIntroduction(t.Context(), 1, time.Now().UTC().Add(60*time.Second).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if introduction.Node(registered) == [32]byte{} || introduction.Slot(registered) == [32]byte{} {
				t.Fatal("registration lost its State recipient or random slot")
			}
			if err := owner.withdrawIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-registered.DoneSignal():
			default:
				t.Fatal("withdrawal did not join registration")
			}
			second, err := owner.registerIntroduction(t.Context(), 2, time.Now().UTC().Add(60*time.Second).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if introduction.Slot(second) == introduction.Slot(registered) {
				t.Fatal("registration reused withdrawn slot")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-second.DoneSignal():
			default:
				t.Fatal("context released a live registration")
			}
			select {
			case <-sourceRouteDone(source):
			default:
				t.Fatal("context released a live Source prefix")
			}
			select {
			case <-introductionPrefix.Done():
			default:
				t.Fatal("context released a live Introduction prefix")
			}
		})
	}
}
