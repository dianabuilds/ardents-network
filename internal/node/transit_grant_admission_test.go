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
	stateSigner := ed25519.NewKeyFromSeed([]byte{8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
		8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8})
	statePublic := stateSigner.Public().(ed25519.PublicKey)
	snapshot := dutyFacts{Generation: "generation-1", NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, NodeID: [32]byte{4}, Assignment: "introduction",
		AssignmentDigest: [32]byte{10}, Fresh: true, ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(time.Minute),
		Authorities: [16]dutyAuthority{{ID: sha256.Sum256(statePublic), PublicKey: [32]byte(statePublic)}}, AuthorityCount: 1,
		TransitGrantSignerID: issuer, TransitGrantSignerPublicKey: [32]byte(public)}
	current := snapshot
	readCurrent := func() (dutyFacts, error) { return current, nil }
	deadline := now.Add(time.Minute)
	grant := route.TransitGrant{IssuerID: issuer, GrantID: [32]byte{5}, NetworkID: snapshot.NetworkID, Digest: snapshot.Digest,
		AttachmentID: [32]byte{6}, TransitNodeID: snapshot.NodeID, ClientKeyDigest: [32]byte{7}, Epoch: snapshot.Epoch,
		TransitRole: route.IntroductionRole, NotAfter: deadline}
	raw, err := route.IssueTransitGrant(grant, signer)
	if err != nil {
		t.Fatal(err)
	}
	admit := stateTransitGrantAdmitter(transitGrantRoot(t), snapshot, readCurrent, func() time.Time { return now })
	if _, err := admit(raw, grant.AttachmentID, grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err != nil {
		t.Fatal(err)
	}
	if _, err := admit(raw, grant.AttachmentID, grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err == nil {
		t.Fatal("replayed State transit grant was accepted")
	}
	otherAttachment := grant.AttachmentID
	otherAttachment[0] = 8
	if _, err := stateTransitGrantAdmitter(t.TempDir(), snapshot, readCurrent, func() time.Time { return now })(raw, otherAttachment,
		grant.ClientKeyDigest, grant.TransitRole, grant.TransitNodeID, deadline); err == nil {
		t.Fatal("substituted attachment was accepted")
	}
	stateGrant := grant
	stateGrant.IssuerID = sha256.Sum256(statePublic)
	stateGrant.GrantID = [32]byte{9}
	stateRaw, err := route.IssueTransitGrant(stateGrant, stateSigner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stateTransitGrantAdmitter(t.TempDir(), snapshot, readCurrent, func() time.Time { return now })(stateRaw, stateGrant.AttachmentID,
		stateGrant.ClientKeyDigest, stateGrant.TransitRole, stateGrant.TransitNodeID, deadline); err == nil {
		t.Fatal("purpose-scoped State rejected an Epoch-authority-signed Transit Grant")
	}
	oldSuccessorGrant := grant
	oldSuccessorGrant.GrantID = [32]byte{15}
	oldSuccessorRaw, err := route.IssueTransitGrant(oldSuccessorGrant, signer)
	if err != nil {
		t.Fatal(err)
	}
	current.Generation = "generation-2"
	current.Digest = [32]byte{30}
	if _, err := admit(oldSuccessorRaw, oldSuccessorGrant.AttachmentID, oldSuccessorGrant.ClientKeyDigest,
		oldSuccessorGrant.TransitRole, oldSuccessorGrant.TransitNodeID, deadline); err == nil {
		t.Fatal("successor State left old-generation Transit Grant admission live")
	}
}

func TestStateTransitGrantAdmitterSeparatesGrantLifetimeFromTLSCertificate(t *testing.T) {
	start := time.Now().UTC().Truncate(time.Second)
	certificate, err := route.NewClientCertificate()
	if err != nil || certificate.Leaf == nil {
		t.Fatalf("new client certificate leaf = %t / %v", certificate.Leaf != nil, err)
	}
	if !start.After(certificate.Leaf.NotBefore) || !start.Before(certificate.Leaf.NotAfter) {
		t.Fatalf("certificate is not valid at test start: [%v,%v) / %v", certificate.Leaf.NotBefore, certificate.Leaf.NotAfter, start)
	}
	clientKey, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	signer := ed25519.NewKeyFromSeed([]byte{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7})
	public := signer.Public().(ed25519.PublicKey)
	stateSigner := ed25519.NewKeyFromSeed([]byte{6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6})
	statePublic := stateSigner.Public().(ed25519.PublicKey)
	snapshot := dutyFacts{Generation: "grant-lifetime", NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, NodeID: [32]byte{4},
		Assignment: "introduction", AssignmentDigest: [32]byte{5}, Fresh: true, ValidUntil: start.Add(10 * time.Minute), RecordValidUntil: start.Add(10 * time.Minute),
		Authorities: [16]dutyAuthority{{ID: sha256.Sum256(statePublic), PublicKey: [32]byte(statePublic)}}, AuthorityCount: 1,
		TransitGrantSignerID: sha256.Sum256(public), TransitGrantSignerPublicKey: [32]byte(public)}
	clock := start
	current := func() (dutyFacts, error) { return snapshot, nil }
	issue := func(id byte, notAfter time.Time) ([]byte, route.TransitGrant) {
		grant := route.TransitGrant{IssuerID: sha256.Sum256(public), GrantID: [32]byte{id}, NetworkID: snapshot.NetworkID, Digest: snapshot.Digest,
			AttachmentID: [32]byte{id + 10}, TransitNodeID: snapshot.NodeID, ClientKeyDigest: clientKey, Epoch: snapshot.Epoch,
			TransitRole: route.IntroductionRole, NotAfter: notAfter}
		raw, issueErr := route.IssueTransitGrant(grant, signer)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		return raw, grant
	}
	admit := func(root string, raw []byte, grant route.TransitGrant, key [32]byte) error {
		_, admitErr := stateTransitGrantAdmitter(root, snapshot, current, func() time.Time { return clock })(raw,
			grant.AttachmentID, key, grant.TransitRole, grant.TransitNodeID, grant.NotAfter)
		return admitErr
	}

	validRaw, validGrant := issue(1, start.Add(time.Minute))
	if err := admit(transitGrantRoot(t), validRaw, validGrant, clientKey); err != nil {
		t.Fatalf("fresh grant admission = %v", err)
	}

	wrongDigestRaw, wrongDigestGrant := issue(2, start.Add(time.Minute))
	wrongKey := clientKey
	wrongKey[0] ^= 0xff
	wrongDigestRoot := transitGrantRoot(t)
	if err := admit(wrongDigestRoot, wrongDigestRaw, wrongDigestGrant, wrongKey); err == nil {
		t.Fatal("wrong client key digest was accepted")
	}
	if err := admit(wrongDigestRoot, wrongDigestRaw, wrongDigestGrant, clientKey); err != nil {
		t.Fatalf("wrong key digest consumed grant: %v", err)
	}

	exactRaw, exactGrant := issue(3, start.Add(2*time.Minute))
	clock = exactGrant.NotAfter
	if !clock.Before(certificate.Leaf.NotAfter) {
		t.Fatalf("certificate expired before exact Grant deadline: %v", certificate.Leaf.NotAfter)
	}
	if err := admit(transitGrantRoot(t), exactRaw, exactGrant, clientKey); err == nil {
		t.Fatal("Grant was accepted exactly at NotAfter")
	}

	afterRaw, afterGrant := issue(4, start.Add(3*time.Minute))
	clock = afterGrant.NotAfter.Add(time.Second)
	if !clock.Before(certificate.Leaf.NotAfter) {
		t.Fatalf("certificate expired before post-deadline check: %v", certificate.Leaf.NotAfter)
	}
	if err := admit(transitGrantRoot(t), afterRaw, afterGrant, clientKey); err == nil {
		t.Fatal("expired unspent Grant was accepted while certificate remained valid")
	}

	clock = start
	replayRaw, replayGrant := issue(5, start.Add(4*time.Minute))
	replayRoot := transitGrantRoot(t)
	if err := admit(replayRoot, replayRaw, replayGrant, clientKey); err != nil {
		t.Fatalf("replay baseline admission = %v", err)
	}
	if err := admit(replayRoot, replayRaw, replayGrant, clientKey); err == nil {
		t.Fatal("replayed Grant was accepted after reopening admitter")
	}
}
