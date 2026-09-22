package route

import (
	"bytes"
	"testing"
	"time"
)

func TestCredentialRelaySenderRequiresExactSelectedIssuer(t *testing.T) {
	setup := credentialRelaySetupFixture()
	setupRaw, err := EncodeCredentialRelaySetup(setup)
	if err != nil || len(setupRaw) == 0 {
		t.Fatalf("CredentialRelaySetup encode = %x, %v", setupRaw, err)
	}
	readyRaw, err := credentialRelayRecord(credentialRelayReadyKind, setup)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := ReadCredentialRelayReady(bytes.NewReader(readyRaw))
	if err != nil || setup.VerifyCredentialRelayReady(ready) != nil {
		t.Fatalf("CredentialRelayReady = %+v, %v", ready, err)
	}
	ready.Setup.IssuerNodeID[0]++
	if err := setup.VerifyCredentialRelayReady(ready); err == nil {
		t.Fatal("CredentialRelayReady accepted a substituted issuer")
	}
}

func TestCredentialRelaySenderKeepsOpaqueRequestSeparateFromResponse(t *testing.T) {
	var request bytes.Buffer
	if err := WriteCredentialRelayEnvelope(&request, CredentialRelayEnvelope{OHTTP: []byte{1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCredentialRelayReady(request.Bytes()); err == nil {
		t.Fatal("credential ready accepted a request envelope")
	}
	responseRaw, err := credentialResponseFixture([]byte{4, 5})
	if err != nil {
		t.Fatal(err)
	}
	response, err := ReadCredentialRelayResponse(bytes.NewReader(responseRaw))
	if err != nil || response.Framing != CredentialOHTTPResponse || !bytes.Equal(response.OHTTP, []byte{4, 5}) {
		t.Fatalf("CredentialRelayResponse = %+v, %v", response, err)
	}
}

func credentialResponseFixture(payload []byte) ([]byte, error) {
	body := make([]byte, 0, 2+1+1+len(Profile)+1+2+len(payload))
	body = appendUint16(body, routeWireVersion)
	body = append(body, credentialRelayResponseKind)
	body = appendProfile(body)
	body = append(body, CredentialOHTTPResponse)
	body = appendUint16(body, uint16(len(payload)))
	body = append(body, payload...)
	return credentialRouteEnvelope(body)
}

func credentialRelaySetupFixture() CredentialRelaySetup {
	return CredentialRelaySetup{NetworkID: identifier(51), Digest: identifier(52), AttachmentID: identifier(53),
		InitiatorNodeID: identifier(54), IssuerNodeID: identifier(55), IssuerNodePublicKey: identifier(56), IssuerProfileDigest: identifier(57), Epoch: 57,
		NotAfter: time.Date(2026, time.August, 26, 14, 0, 0, 0, time.UTC), EnvelopeCapacity: CredentialEnvelopeCapacity}
}
