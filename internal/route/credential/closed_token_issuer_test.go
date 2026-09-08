package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestClosedTokenIssuerReconcilesCommittedBatchAfterRestart(t *testing.T) {
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	now := window.Add(time.Minute)
	nodePublic, nodePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network, issuerNode := sha256.Sum256([]byte("issuer network")), sha256.Sum256([]byte("issuer node"))
	root := t.TempDir()
	receipt, err := InitializeClosedIssuerRoot(ClosedIssuerRootConfig{Root: root, NetworkID: network, NodeID: issuerNode, IdentityKey: nodePrivate,
		NotBefore: window, NotAfter: window.Add(time.Hour), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	issuerProfile, err := DecodeClosedIssuerProfile(receipt.Profile, nodePublic)
	if err != nil {
		t.Fatal(err)
	}
	authority := ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(2))
	profile := state.ClosedProfileView{Digest: sha256.Sum256([]byte("accepted State profile")), IssuerNodeID: issuerNode,
		IssuerDutyGeneration: 4, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: uint8(len(issuerProfile.Keys))}
	copy(profile.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	for index, key := range issuerProfile.Keys {
		profile.TokenKeys[index].WindowStart, profile.TokenKeys[index].Class = key.WindowStart, uint8(key.Class)
		copy(profile.TokenKeys[index].SPKI[:], key.SPKI)
	}
	holder := ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(3))
	permission := Permission{NetworkID: network, IssuerNodeID: issuerNode, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: sha256.Sum256([]byte("issuer permission")), NotBefore: window, NotAfter: window.Add(time.Hour), Maxima: [3]uint32{2, 0, 0}}
	copy(permission.HolderKey[:], holder.Public().(ed25519.PublicKey))
	copy(permission.Signature[:], ed25519.Sign(authority, permissionTranscript(permission)))
	context := ClosedTokenContext{NetworkID: network, ProfileDigest: profile.Digest, ReceiverNodeID: sha256.Sum256([]byte("receiver")),
		IssuerNodeID: issuerNode, ReceiverDutyGeneration: 6, Class: 1, WindowStart: window}
	pending, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Context: context, Permission: permission, HolderKey: holder, Count: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	raw := pending.Request()
	current := func() (state.ClosedProfileView, bool) { return profile, true }
	open := func() *ClosedTokenIssuer {
		t.Helper()
		issuer, err := OpenClosedTokenIssuer(ClosedTokenIssuerConfig{Root: root, NetworkID: network, CurrentProfile: current, Clock: func() time.Time { return now }})
		if err != nil {
			t.Fatal(err)
		}
		return issuer
	}
	issuer := open()
	defer func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	}()
	decoded, err := DecodeClosedTokenBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !issuer.profileCurrent() {
		t.Fatal("opened issuer rejected its current State profile")
	}
	if err := VerifyPermission(decoded.Permission, ed25519.PublicKey(profile.IssuanceAuthorityKey[:]), network, issuerNode, profile.IssuerDutyGeneration, now); err != nil {
		t.Fatalf("issuer permission precondition: %v", err)
	}
	if _, err := issuer.privateKey(decoded); err != nil {
		t.Fatalf("issuer key precondition: %v", err)
	}
	firstRaw := issuer.IssueEncoded(raw)
	first, err := DecodeClosedTokenBatchResult(firstRaw)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != ClosedTokenIssued || len(first.Signatures) != 2 {
		t.Fatalf("first issuer result = %+v", first)
	}
	if tokens, err := pending.FinalizeEncoded(firstRaw); err != nil || len(tokens) != 2 || len(tokens[0]) != closedTokenSize {
		t.Fatalf("finalize committed batch = %d tokens / %v", len(tokens), err)
	}
	if err := issuer.Close(); err != nil {
		t.Fatal(err)
	}
	issuer = open()
	defer func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	}()
	retriedRaw := issuer.IssueEncoded(raw)
	retried, err := DecodeClosedTokenBatchResult(retriedRaw)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != ClosedTokenIssued || len(retried.Signatures) != len(first.Signatures) || !bytes.Equal(retried.Signatures[0], first.Signatures[0]) ||
		!bytes.Equal(retried.Signatures[1], first.Signatures[1]) || !bytes.Equal(retriedRaw, firstRaw) {
		t.Fatalf("restarted issuer result = %+v", retried)
	}
	second, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Context: context, Permission: permission, HolderKey: holder, Count: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Discard()
	if exhausted := issuer.Issue(second.Request()); exhausted.Status != ClosedTokenExhausted {
		t.Fatalf("permission overflow result = %+v", exhausted)
	}
}
