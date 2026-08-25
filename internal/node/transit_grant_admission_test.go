package node

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestStateTransitGrantAdmitterConsumesOnlyOneExactCurrentGrant(t *testing.T) {
	now := time.Unix(1_750_000_000, 0).UTC()
	signer := ed25519.NewKeyFromSeed([]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9,
		9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
	public := signer.Public().(ed25519.PublicKey)
	issuer := sha256.Sum256(public)
	snapshot := dutyFacts{NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, NodeID: [32]byte{4}, Assignment: "introduction",
		Authorities: [16]dutyAuthority{{ID: issuer, PublicKey: [32]byte(public)}}, AuthorityCount: 1}
	deadline := now.Add(time.Minute)
	grant := route.TransitGrant{IssuerID: issuer, GrantID: [32]byte{5}, NetworkID: snapshot.NetworkID, Digest: snapshot.Digest,
		AttachmentID: [32]byte{6}, TransitNodeID: snapshot.NodeID, ClientKeyDigest: [32]byte{7}, Epoch: snapshot.Epoch,
		TransitRole: route.IntroductionRole, NotAfter: deadline}
	raw, err := route.IssueTransitGrant(grant, signer)
	if err != nil {
		t.Fatal(err)
	}
	admit := stateTransitGrantAdmitter(t.TempDir(), snapshot, func() time.Time { return now })
	if _, err := admit(raw, grant.AttachmentID, grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err != nil {
		t.Fatal(err)
	}
	if _, err := admit(raw, grant.AttachmentID, grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err == nil {
		t.Fatal("replayed State transit grant was accepted")
	}
	otherAttachment := grant.AttachmentID
	otherAttachment[0] = 8
	if _, err := stateTransitGrantAdmitter(t.TempDir(), snapshot, func() time.Time { return now })(raw, otherAttachment,
		grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err == nil {
		t.Fatal("substituted attachment was accepted")
	}
}
