package route

import (
	"bytes"
	"testing"
	"time"
)

func TestCredentialRelayRequiresExactSelectedIssuer(t *testing.T) {
	setup := credentialRelaySetupFixture()
	raw, err := EncodeCredentialRelaySetup(setup)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCredentialRelaySetup(raw)
	if err != nil || decoded != setup {
		t.Fatalf("CredentialRelaySetup = %+v, %v", decoded, err)
	}
	readyRaw, err := EncodeCredentialRelayReady(CredentialRelayReady{Setup: setup})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := DecodeCredentialRelayReady(readyRaw)
	if err != nil || setup.VerifyCredentialRelayReady(ready) != nil {
		t.Fatalf("CredentialRelayReady = %+v, %v", ready, err)
	}
	ready.Setup.IssuerNodeID[0]++
	if err := setup.VerifyCredentialRelayReady(ready); err == nil {
		t.Fatal("CredentialRelayReady accepted a substituted issuer")
	}
	for _, malformed := range [][]byte{raw[:len(raw)-1], append(raw, 0)} {
		if _, err := DecodeCredentialRelaySetup(malformed); err == nil {
			t.Fatal("CredentialRelaySetup accepted malformed bytes")
		}
	}
}

func TestCredentialRelayIOKeepsOpaqueRequestSeparateFromResponse(t *testing.T) {
	setup := credentialRelaySetupFixture()
	var wire bytes.Buffer
	if err := WriteCredentialRelaySetup(&wire, setup); err != nil {
		t.Fatal(err)
	}
	if err := WriteCredentialRelayEnvelope(&wire, CredentialRelayEnvelope{OHTTP: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCredentialRelaySetup(&wire)
	if err != nil || got != setup {
		t.Fatalf("CredentialRelaySetup IO = %+v, %v", got, err)
	}
	envelope, err := ReadCredentialRelayEnvelope(&wire)
	if err != nil || !bytes.Equal(envelope.OHTTP, []byte{1, 2, 3}) {
		t.Fatalf("CredentialRelayEnvelope IO = %x, %v", envelope.OHTTP, err)
	}
	responseRaw, err := encodeCredentialRelayResponse(CredentialRelayResponse{OHTTP: []byte{4, 5}, Framing: CredentialOHTTPResponse})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCredentialRelayEnvelope(responseRaw, credentialRelayEnvelopeKind); err == nil {
		t.Fatal("credential response decoded as request")
	}
}

func credentialRelaySetupFixture() CredentialRelaySetup {
	return CredentialRelaySetup{NetworkID: identifier(51), Digest: identifier(52), AttachmentID: identifier(53),
		InitiatorNodeID: identifier(54), IssuerNodeID: identifier(55), IssuerNodePublicKey: identifier(56), IssuerProfileDigest: identifier(57), Epoch: 57,
		NotAfter: time.Date(2026, time.August, 26, 14, 0, 0, 0, time.UTC), EnvelopeCapacity: CredentialEnvelopeCapacity}
}
