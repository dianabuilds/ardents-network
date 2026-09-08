package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestPrepareClosedTokenBatchBindsStatePermissionAndVolatileBlindState(t *testing.T) {
	issuerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := encodeSelectedRSAPSSSPKI(&issuerKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	authority := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	holder := ed25519.NewKeyFromSeed(bytesForClosedTokenBatch(1))
	profile := state.ClosedProfileView{Digest: sha256.Sum256([]byte("profile")), IssuerNodeID: sha256.Sum256([]byte("issuer")),
		IssuerDutyGeneration: 9, TokenKeyCount: 1}
	copy(profile.IssuanceAuthorityKey[:], authority.Public().(ed25519.PublicKey))
	profile.TokenKeys[0] = state.ClosedProfileTokenKey{WindowStart: window, Class: 2}
	copy(profile.TokenKeys[0].SPKI[:], spki)
	permission := Permission{NetworkID: sha256.Sum256([]byte("network")), IssuerNodeID: profile.IssuerNodeID, DutyGeneration: profile.IssuerDutyGeneration,
		PermissionID: sha256.Sum256([]byte("permission")), NotBefore: window, NotAfter: window.Add(time.Hour), Maxima: [3]uint32{0, 3, 0}}
	copy(permission.HolderKey[:], holder.Public().(ed25519.PublicKey))
	copy(permission.Signature[:], ed25519.Sign(authority, permissionTranscript(permission)))
	context := ClosedTokenContext{NetworkID: permission.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: sha256.Sum256([]byte("receiver")),
		IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: 8, Class: 2, WindowStart: window}
	pending, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Context: context, Permission: permission, HolderKey: holder, Count: 3, Now: window.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	raw := pending.Request()
	request, err := DecodeClosedTokenBatch(raw)
	if err != nil || request.Permission != permission || request.RequestID == [32]byte{} || request.Class != 2 || request.WindowStart != window ||
		len(request.BlindedRequests) != 3 || request.SPKI != profile.TokenKeys[0].SPKI {
		t.Fatalf("decode closed token batch = %+v / %v", request, err)
	}
	keyID := sha256.Sum256(spki)
	for _, blinded := range request.BlindedRequests {
		if len(blinded) != closedTokenRequestSize || binary.BigEndian.Uint16(blinded[:2]) != closedTokenType || blinded[2] != keyID[31] {
			t.Fatalf("token request = %x", blinded)
		}
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-ed25519.SignatureSize-1] ^= 1
	if _, err := DecodeClosedTokenBatch(tampered); err == nil {
		t.Fatal("accepted a changed blinded batch body")
	}
	pending.Discard()
	if pending.Request() != nil {
		t.Fatal("discard retained a retryable request")
	}
	if _, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Context: context, Permission: permission, HolderKey: authority, Count: 3, Now: window.Add(time.Minute)}); err == nil {
		t.Fatal("accepted a holder key different from the permission")
	}
}

func bytesForClosedTokenBatch(value byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	return seed
}
