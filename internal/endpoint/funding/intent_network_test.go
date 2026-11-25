package funding_test

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// The draft is checked against a State owner that accepted independently signed
// Network bytes; it is not a test of an executed issuance or a CLI operation.
func TestNetworkIssuanceDraftRetainsExactBindingsAndHorizons(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, root)
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal("signed Network", err)
	}
	clock, stopClock := networkinputs.StartObservedClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := networkinputs.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	facts, _, err := funding.NetworkFacts(view)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	deadline := view.ObservedAt().Add(time.Second).UTC().Truncate(time.Second)
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{{NetworkID: facts.NetworkID,
		ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: receiver.NodeID,
		ReceiverDutyGeneration: receiver.DutyGeneration, Class: 2, WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)}},
		Selection: stock.ExchangeBinding{ID: [32]byte{1}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: deadline}
	if err := funding.ValidateIntent(view, intent); err != nil {
		t.Fatal("valid same-observation draft refused", err)
	}
	for _, boundary := range []string{"profile", "network", "issuer", "receiver", "duty", "expired", "horizon"} {
		t.Run(boundary, func(t *testing.T) {
			changed := intent
			changed.Challenges = append([]token.ClosedTokenContext(nil), intent.Challenges...)
			switch boundary {
			case "profile":
				changed.Selection.ProfileDigest[0] ^= 1
			case "network":
				changed.Challenges[0].NetworkID[0] ^= 1
			case "issuer":
				changed.Challenges[0].IssuerNodeID[0] ^= 1
			case "receiver":
				changed.Challenges[0].ReceiverNodeID = [32]byte{255}
			case "duty":
				changed.Challenges[0].ReceiverDutyGeneration++
			case "expired":
				changed.Deadline = view.ObservedAt()
			case "horizon":
				changed.Deadline = facts.NotAfter.Add(time.Second)
			}
			if err := funding.ValidateIntent(view, changed); err == nil {
				t.Fatal("changed observed issuance binding accepted")
			}
		})
	}
}
