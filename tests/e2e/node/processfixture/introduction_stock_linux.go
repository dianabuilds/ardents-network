//go:build linux && node_process_native

package processfixture

import (
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// IntroductionStock funds a Publisher with separate classes for one native Introduction exchange.
func IntroductionStock(t *testing.T, spec networkfixture.ClosedSpec, plan issuer.Plan, read func() (network.RuntimeView, error)) *stock.Owner {
	t.Helper()
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := read()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	profile, now, err := observe()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, admission.AllocationPublisher, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request([3]uint32{1, 8, 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, FixturePermission(t, raw, profile, spec.Authority, now)); err != nil {
		t.Fatal(err)
	}
	var batches [3][]token.ClosedTokenContext
	challenge := func(id byte, class uint8) token.ClosedTokenContext {
		return token.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest,
			IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9,
			Class: class, WindowStart: now.UTC().Truncate(time.Hour)}
	}
	for _, id := range []byte{12, 13, 14, 15, 21, 22, 23, 24} {
		batches[0] = append(batches[0], challenge(id, 2))
	}
	batches[1], batches[2] = []token.ClosedTokenContext{challenge(25, 3)}, []token.ClosedTokenContext{challenge(25, 1)}
	for i, contexts := range batches {
		bootstrap, kind := i < 2, quota.Admitted
		if bootstrap {
			kind = quota.Bootstrap
		}
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: contexts, Selection: stock.ExchangeBinding{ID: [32]byte{byte(60 + i)}, ProfileDigest: profile.Digest}, Bootstrap: bootstrap, Deadline: profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		request, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		// Offline fixture issuance remains explicit, not a network issuer ACK.
		issued := issuer.IssueCurrent(t.Context(), plan, request, kind, observe)
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	return holder
}
