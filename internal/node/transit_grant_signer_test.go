package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestNodeProjectsOnlyPurposeScopedSignerFromStateIssuerProfile(t *testing.T) {
	now := time.Unix(2_000_700_000, 0).UTC()
	network, digest, issuerID := [32]byte{1}, [32]byte{2}, [32]byte{3}
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	grantPublic, grantPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	initiatorPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := credential.NewIssuer(credential.IssuerConfig{NetworkID: network, NodeID: issuerID, IdentityKey: issuerPrivate,
		GrantSigner: grantPrivate, InitiatorNodeID: [32]byte{4}, InitiatorPublicKey: [32]byte(initiatorPublic),
		DutyRoot: t.TempDir(), CreateDutyRoot: true, Budget: 2, Clock: func() time.Time { return now },
		CurrentDuty: func() (credential.StateDuty, bool) {
			return credential.StateDuty{NetworkID: network, Digest: digest, IssuerNodeID: issuerID,
				IssuerPublicKey: [32]byte(issuerPublic), InitiatorNodeID: [32]byte{4}, InitiatorPublicKey: [32]byte(initiatorPublic),
				GrantSignerPublicKey: [32]byte(grantPublic), Epoch: 5, NotAfter: now.Add(time.Minute)}, true
		}, Authorize: func(credential.Request, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = issuer.Close() }()
	profile, err := credential.EncodeProfile(issuer.Profile())
	if err != nil {
		t.Fatal(err)
	}
	statePublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := dutyFacts{NetworkID: network, Epoch: 5, Digest: digest, ValidUntil: now.Add(time.Minute),
		TransitIssuerNodeID: issuerID, TransitIssuerProfile: profile, CandidateCount: 1,
		Candidates: [64]dutyCandidate{{NodeID: issuerID, PublicKey: [32]byte(issuerPublic), Assignment: "transit-issuance",
			ValidUntil: now.Add(time.Minute), AssignmentNotAfter: now.Add(time.Minute)}},
		Authorities: [16]dutyAuthority{{ID: sha256.Sum256(statePublic), PublicKey: [32]byte(statePublic)}}, AuthorityCount: 1}
	if err := attachTransitGrantSigner(&snapshot, now); err != nil {
		t.Fatal(err)
	}
	if snapshot.TransitGrantSignerID != sha256.Sum256(grantPublic) || snapshot.TransitGrantSignerPublicKey != [32]byte(grantPublic) {
		t.Fatalf("projected signer = (%x, %x)", snapshot.TransitGrantSignerID, snapshot.TransitGrantSignerPublicKey)
	}
	if snapshot.TransitGrantSignerID == snapshot.Authorities[0].ID {
		t.Fatal("purpose-scoped Grant signer collapsed into the Epoch authority")
	}

	changed := snapshot
	changed.TransitGrantSignerID, changed.TransitGrantSignerPublicKey = [32]byte{}, [32]byte{}
	changed.TransitIssuerProfile = append([]byte(nil), profile...)
	changed.TransitIssuerProfile[len(changed.TransitIssuerProfile)-1] ^= 1
	if err := attachTransitGrantSigner(&changed, now); err == nil {
		t.Fatal("Node accepted a substituted State issuer profile")
	}
}
