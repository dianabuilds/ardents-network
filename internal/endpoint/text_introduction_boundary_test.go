//go:build linux

package endpoint

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// This wrapper pauses a synchronous authority read at the last binding
// boundary. Node runtimes and the already opened prefixes retain real State.
type textCapsuleBoundaryState struct {
	*textSourceStateFixture
	reads     int
	atRead    int
	atBinding func()
}

func (source *textCapsuleBoundaryState) CurrentClosedProfile() (state.ClosedProfileView, error) {
	source.reads++
	atRead := source.atRead
	if atRead == 0 {
		atRead = 2
	}
	if source.reads == atRead && source.atBinding != nil {
		source.atBinding()
	}
	return source.textSourceStateFixture.CurrentClosedProfile()
}

func checkTextCapsuleAdmissionBoundaries(t *testing.T, publisher, reader *textContext, source *textSourceStateFixture,
	job *textJobIdentity, original *textIntroductionAttempt, recipient [32]byte) {
	t.Helper()
	endpoint := publisher.endpoint
	for index, name := range []string{"resolution family", "Introduction family", "issuer family", "caller cancellation", "deadline"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			clock := endpoint.clock
			defer func() { endpoint.clock, endpoint.closedState = clock, source }()
			plaintext := original.plaintext
			plaintext.Deadline = clock().Add(10 * time.Second).UTC().Truncate(time.Second)
			capsule := route.ClosedIntroductionCapsule{Slot: publisher.registration.request.Slot, Revision: plaintext.Revision,
				Expiry: plaintext.Deadline, DeliveryNonce: fixtureID(byte(140 + index))}
			sealed, _, err := route.SealClosedIntroduction(capsule, recipient, plaintext)
			if err != nil {
				t.Fatal(err)
			}
			operation, err := route.EncodeClosedIntroductionSubmission(fixtureID(byte(150+index)), sealed)
			if err != nil {
				t.Fatal(err)
			}
			// Each case isolates admission from the separately tested rate limiter.
			publisher.introductionOpenings = [4]time.Time{}
			if index < 3 {
				source.mu.Lock()
				rendezvous := -1
				for i, candidate := range source.snapshot.Candidates[:source.snapshot.CandidateCount] {
					if candidate.NodeID == original.plaintext.RendezvousNode {
						rendezvous = i
					}
				}
				if rendezvous < 0 {
					source.mu.Unlock()
					t.Fatal("missing data Rendezvous fixture")
				}
				prior := source.snapshot.Candidates[rendezvous].FamilyID
				var conflict [32]byte
				for _, role := range source.view.Nodes[:source.view.NodeCount] {
					purpose := []route.ClosedPurpose{route.ClosedPurposeReachability, route.ClosedPurposeIntroduction, route.ClosedPurposeIssuer}[index]
					if route.ClosedPurposePermitsDuty(purpose, role.RoleDomain, role.Subrole) {
						for _, candidate := range source.snapshot.Candidates[:source.snapshot.CandidateCount] {
							if candidate.NodeID == role.NodeID {
								conflict = candidate.FamilyID
							}
						}
					}
				}
				if conflict == [32]byte{} {
					source.mu.Unlock()
					t.Fatal("missing control duty fixture")
				}
				source.snapshot.Candidates[rendezvous].FamilyID = conflict
				source.mu.Unlock()
				defer func() { source.mu.Lock(); source.snapshot.Candidates[rendezvous].FamilyID = prior; source.mu.Unlock() }()
				for _, owner := range []*textContext{reader, publisher} {
					if _, _, _, err := owner.prefix.DataJoinRecipient(); err == nil {
						t.Error("data Rendezvous accepted a control-role family")
					}
				}
			} else {
				endpoint.closedState = &textCapsuleBoundaryState{textSourceStateFixture: source, atBinding: func() {
					if index == 3 {
						cancel()
					} else {
						endpoint.clock = func() time.Time { return capsule.Expiry }
					}
				}}
			}
			accepted, err := publisher.acceptTextIntroduction(ctx, job, operation)
			if err == nil || accepted != nil || strings.Contains(err.Error(), "rate unavailable") {
				t.Errorf("invalid final admission accepted: %v", err)
			}
			if _, retained := publisher.introductionReplays[capsule.DeliveryNonce]; retained {
				t.Error("failed admission retained a successful delivery")
			}
		})
	}
}
