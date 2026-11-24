//go:build linux

package main

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
)

func TestHolderSessionRetainsOriginalIssuanceRecipients(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{0, 1, 0})
	conversation, err := session.OpenLocal(holder, fundingAuthority(f.authority), session.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conversation.Close(); err != nil {
			t.Error(err)
		}
	})
	challenge := token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest,
		IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: f.receiver.NodeID,
		ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: uint8(admission.ForwardClass),
		WindowStart: time.Now().UTC().Truncate(time.Hour)}
	intent := stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge},
		Selection: stock.ExchangeBinding{ID: [32]byte{21}, ProfileDigest: f.profile.Digest},
		Bootstrap: true, Deadline: f.profile.NotAfter}
	batch, _, err := conversation.Begin(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	// The caller can reuse its input after Begin. Completion must reobserve the
	// original recipient, independently of this now-invalid caller-owned draft.
	intent.Challenges[0].ReceiverNodeID = [32]byte{255}
	intent.Challenges[0].ReceiverDutyGeneration++
	issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
		return f.authority.Issuer(f.plan.KeyBinding.Signer)
	})
	if issued.Outcome != "issued-offline" {
		t.Fatal(issued.Outcome)
	}
	if err := conversation.Complete(t.Context(), issued.Response, false); err != nil {
		t.Fatal("caller mutation changed retained issuance validation", err)
	}
	presentation := stock.Presentation{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration,
		StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, RecipientNodeID: f.receiver.NodeID,
		RecipientDutyGeneration: f.receiver.DutyGeneration, ChannelNonce: [32]byte{22}, Deadline: f.profile.NotAfter}
	raw, err := conversation.Take(t.Context(), presentation, uint8(admission.ForwardClass))
	defer clear(raw)
	if err != nil || len(raw) == 0 {
		t.Fatal("original recipient lost issued right", err)
	}
}
