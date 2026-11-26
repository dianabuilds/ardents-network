//go:build linux || darwin

package session_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestHolderSessionRetainsOriginalIssuanceRecipients(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t)
	facts, now, err := fixture.ObserveIssuer()
	if err != nil {
		t.Fatal(err)
	}
	receiver := fixture.Spec.Nodes[1]
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, admission.AllocationUser, fixture.ObserveIssuer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() })
	request, digest, err := holder.Request([3]uint32{0, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(request)
	permission := processfixture.FixturePermission(t, request, facts, fixture.Spec.Authority, now)
	defer clear(permission)
	if err := holder.Import(digest, permission); err != nil {
		t.Fatal(err)
	}
	current := fixture.State.CurrentRuntime
	authority := funding.Observe(current, fixture.State.CurrentRuntimeWitness, funding.PresentationGuard(current))
	conversation, err := session.OpenLocal(holder, authority, session.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conversation.Close(); err != nil {
			t.Error(err)
		}
	})
	challenge := token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest,
		IssuerNodeID: facts.IssuerNodeID, ReceiverNodeID: receiver.NodeID,
		ReceiverDutyGeneration: receiver.Generation, Class: uint8(admission.ForwardClass),
		WindowStart: now.UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge},
		Selection: stock.ExchangeBinding{ID: [32]byte{21}, ProfileDigest: facts.Digest},
		Bootstrap: true, Deadline: facts.NotAfter}
	batch, _, err := conversation.Begin(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	// The caller can reuse its input after Begin. Completion must reobserve the
	// original recipient, independently of this now-invalid caller-owned draft.
	intent.Challenges[0].ReceiverNodeID = [32]byte{255}
	intent.Challenges[0].ReceiverDutyGeneration++
	defer clear(batch)
	issued := issuer.IssueCurrent(t.Context(), fixture.Plan, batch, quota.Bootstrap, fixture.ObserveIssuer)
	defer clear(issued.Response)
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued.Outcome)
	}
	if err := conversation.Complete(t.Context(), issued.Response, false); err != nil {
		t.Fatal("caller mutation changed retained issuance validation", err)
	}
	presentation := stock.Presentation{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration,
		StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, RecipientNodeID: receiver.NodeID,
		RecipientDutyGeneration: receiver.Generation, ChannelNonce: [32]byte{22}, Deadline: facts.NotAfter}
	raw, err := conversation.Take(t.Context(), presentation, uint8(admission.ForwardClass))
	defer clear(raw)
	if err != nil {
		t.Fatal("original recipient lost issued right", err)
	}
	for _, key := range facts.TokenKeys[:facts.TokenKeyCount] {
		if key.Class == challenge.Class && key.WindowStart.Equal(challenge.WindowStart) {
			if err := token.VerifyClosedToken(challenge, key.SPKI[:], raw); err != nil {
				t.Fatal("issued right does not verify for the original recipient", err)
			}
			return
		}
	}
	t.Fatal("original recipient has no matching signed token key")
}
