//go:build linux

package processfixture

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// FundRouteStock uses genuine bounded bootstrap issuance to stock the signed
// Forward receivers and, when named, two Control operations on one Descriptor.
func FundRouteStock(t *testing.T, spec networkfixture.ClosedSpec, bundle networkfixture.Closed, plan issuer.Plan, read func() (network.RuntimeView, error), role admission.AllocationRole, descriptor [32]byte) *stock.Owner {
	t.Helper()
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := read()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	facts, now, err := observe()
	if err != nil {
		t.Fatal(err)
	}
	var forwards, controls []token.ClosedTokenContext
	for _, member := range bundle.Nodes[1:] {
		challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
			IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.Generation,
			Class: uint8(admission.ForwardClass), WindowStart: now.UTC().Truncate(time.Hour)}
		switch {
		case descriptor != [32]byte{} && member.NodeID == descriptor:
			challenge.Class = uint8(admission.ControlClass)
			controls = append(controls, challenge, challenge)
		case member.Subrole == 1 || member.Subrole == 2 || member.Subrole == 4:
			forwards = append(forwards, challenge)
		}
	}
	if descriptor != [32]byte{} && len(controls) != 2 {
		t.Fatal("signed Descriptor missing from Route Stock fixture")
	}
	maxima := [3]uint32{uint32(len(controls)), uint32(len(forwards)), 0}
	holder := OpenPermittedStock(t, t.TempDir(), role, maxima, observe, facts, spec.Authority, now)
	if status := holder.Status(); !status.Accepted || status.Remaining != maxima {
		t.Fatalf("Route allocation maxima changed: %+v", status)
	}
	for i, challenges := range [][]token.ClosedTokenContext{forwards, controls} {
		if len(challenges) == 0 {
			continue
		}
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{31, byte(role), byte(i)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		class := challenges[0].Class
		decoded, err := admission.DecodeClosedTokenBatch(batch)
		if err != nil || decoded.Class != class || len(decoded.BlindedRequests) != len(challenges) {
			t.Fatal("Route blinded batch class/count", err)
		}
		issued := issuer.IssueCurrent(t.Context(), plan, batch, quota.Bootstrap, observe)
		if issued.Outcome != "issued-offline" {
			t.Fatal("Route bootstrap issuance", issued.Outcome)
		}
		response, err := admission.DecodeClosedTokenBatchResult(issued.Response)
		if err != nil || response.Status != admission.ClosedTokenIssued || len(response.Signatures) != len(challenges) {
			t.Fatal("Route blind issuance result count", err)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}
