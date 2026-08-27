package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestAlphaBrowserServiceAttachmentUsesOnlyCurrentSignedGrant(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network, digest := alphaBrowserGrantID(1), alphaBrowserGrantID(2)
	introduction, attachment := alphaBrowserGrantID(3), alphaBrowserGrantID(4)
	issuer := sha256.Sum256(public)
	grant := route.TransitGrant{IssuerID: issuer, GrantID: alphaBrowserGrantID(5), NetworkID: network, Digest: digest,
		AttachmentID: attachment, TransitNodeID: introduction, ClientKeyDigest: alphaBrowserGrantID(6), Epoch: 7,
		TransitRole: route.IntroductionRole, NotAfter: now.Add(time.Minute)}
	raw, err := route.IssueTransitGrant(grant, private)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], public)
	epoch := state.ResolutionEpoch{NetworkID: network, Number: 7, Digest: digest,
		Authorities: []state.ResolutionAuthority{{ID: issuer, PublicKey: authority}}}

	got, err := alphaBrowserServiceAttachment(raw, epoch, introduction, now.Add(30*time.Second))
	if err != nil || got != attachment {
		t.Fatalf("signed Transit Grant attachment = %x, %v; want %x", got, err, attachment)
	}
	if _, err := alphaBrowserServiceAttachment(raw, epoch, alphaBrowserGrantID(7), now.Add(30*time.Second)); err == nil {
		t.Fatal("Transit Grant for another Introduction Node was accepted")
	}
	if _, err := alphaBrowserServiceAttachment(raw, state.ResolutionEpoch{NetworkID: network, Number: 7, Digest: digest}, introduction, now.Add(30*time.Second)); err == nil {
		t.Fatal("Transit Grant without a current State authority was accepted")
	}
}

func alphaBrowserGrantID(marker byte) [32]byte {
	var value [32]byte
	for index := range value {
		value[index] = marker
	}
	return value
}
