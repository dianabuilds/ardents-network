package route

import (
	"bytes"
	"testing"
	"time"
)

func TestResolutionRelaySenderRequiresExactReady(t *testing.T) {
	setup := resolutionRelaySetupFixture()
	setupRaw, err := EncodeResolutionRelaySetup(setup)
	if err != nil || len(setupRaw) == 0 {
		t.Fatalf("ResolutionRelaySetup encode = %x, %v", setupRaw, err)
	}
	readyRaw, err := resolutionRelayRecord(resolutionRelayReadyKind, setup)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := ReadResolutionRelayReady(bytes.NewReader(readyRaw))
	if err != nil || setup.VerifyResolutionRelayReady(ready) != nil {
		t.Fatalf("ResolutionRelayReady = %+v, %v", ready, err)
	}
	ready.Setup.GatewayNodeID[0]++
	if err := setup.VerifyResolutionRelayReady(ready); err == nil {
		t.Fatal("ResolutionRelaySetup accepted substituted Gateway confirmation")
	}
}

func TestResolutionRelaySenderKeepsBoundedRequestAndResponseKindsSeparate(t *testing.T) {
	payload := bytes.Repeat([]byte{0xa5}, ResolutionEnvelopeCapacity)
	requestRaw, err := EncodeResolutionRelayEnvelope(ResolutionRelayEnvelope{OHTTP: payload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeResolutionRelayResponse(requestRaw); err == nil {
		t.Fatal("resolution response accepted request kind")
	}
	responseRaw, err := resolutionResponseFixture([]byte{7, 8}, ResolutionOHTTPChunkedResponse)
	if err != nil {
		t.Fatal(err)
	}
	response, err := ReadResolutionRelayResponse(bytes.NewReader(responseRaw))
	if err != nil || response.Framing != ResolutionOHTTPChunkedResponse || !bytes.Equal(response.OHTTP, []byte{7, 8}) {
		t.Fatalf("ResolutionRelayResponse = %+v, %v", response, err)
	}
	for _, value := range []ResolutionRelayEnvelope{{}, {OHTTP: bytes.Repeat([]byte{1}, ResolutionEnvelopeCapacity+1)}} {
		if _, err := EncodeResolutionRelayEnvelope(value); err == nil {
			t.Fatal("resolution relay sender accepted invalid opaque capacity")
		}
	}
}

func resolutionResponseFixture(payload []byte, framing byte) ([]byte, error) {
	body := make([]byte, 0, 2+1+1+len(Profile)+1+2+len(payload))
	body = appendUint16(body, routeWireVersion)
	body = append(body, resolutionRelayResponseKind)
	body = appendProfile(body)
	body = append(body, framing)
	body = appendUint16(body, uint16(len(payload)))
	body = append(body, payload...)
	return resolutionRouteEnvelope(body)
}

func resolutionRelaySetupFixture() ResolutionRelaySetup {
	return ResolutionRelaySetup{NetworkID: identifier(41), Digest: identifier(42), AttachmentID: identifier(43),
		InitiatorNodeID: identifier(44), GatewayNodeID: identifier(45), GatewayNodePublicKey: identifier(46), Epoch: 47,
		NotAfter: time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC), EnvelopeCapacity: ResolutionEnvelopeCapacity}
}
