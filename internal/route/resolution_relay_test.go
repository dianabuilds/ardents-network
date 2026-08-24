package route

import (
	"bytes"
	"testing"
	"time"
)

func TestResolutionRelaySetupRequiresExactReady(t *testing.T) {
	setup := resolutionRelaySetupFixture()
	raw, err := EncodeResolutionRelaySetup(setup)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResolutionRelaySetup(raw)
	if err != nil || decoded != setup {
		t.Fatalf("ResolutionRelaySetup = %+v, %v", decoded, err)
	}
	readyRaw, err := EncodeResolutionRelayReady(ResolutionRelayReady{Setup: setup})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := DecodeResolutionRelayReady(readyRaw)
	if err != nil || setup.VerifyResolutionRelayReady(ready) != nil {
		t.Fatalf("ResolutionRelayReady = %+v, %v, %v", ready, err, setup.VerifyResolutionRelayReady(ready))
	}
	ready.Setup.GatewayNodeID[0]++
	if err := setup.VerifyResolutionRelayReady(ready); err == nil {
		t.Fatal("ResolutionRelaySetup accepted substituted Gateway confirmation")
	}
	for _, value := range [][]byte{raw[:len(raw)-1], append(raw, 0)} {
		if _, err := DecodeResolutionRelaySetup(value); err == nil {
			t.Fatal("ResolutionRelaySetup accepted malformed bytes")
		}
	}
}

func TestResolutionRelayEnvelopeHasOneBoundedOpaqueCapacity(t *testing.T) {
	payload := bytes.Repeat([]byte{0xa5}, ResolutionEnvelopeCapacity)
	raw, err := EncodeResolutionRelayEnvelope(ResolutionRelayEnvelope{OHTTP: payload})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResolutionRelayEnvelope(raw)
	if err != nil || !bytes.Equal(decoded.OHTTP, payload) {
		t.Fatalf("ResolutionRelayEnvelope length = %d, %v", len(decoded.OHTTP), err)
	}
	if _, err := DecodeResolutionRelayResponse(raw); err == nil {
		t.Fatal("resolution response accepted request kind")
	}
	responseRaw, err := EncodeResolutionRelayResponse(ResolutionRelayResponse{OHTTP: []byte{7, 8}, Framing: ResolutionOHTTPChunkedResponse})
	if err != nil {
		t.Fatal(err)
	}
	response, err := DecodeResolutionRelayResponse(responseRaw)
	if err != nil || response.Framing != ResolutionOHTTPChunkedResponse || !bytes.Equal(response.OHTTP, []byte{7, 8}) {
		t.Fatalf("ResolutionRelayResponse = %+v, %v", response, err)
	}
	for _, value := range []ResolutionRelayEnvelope{{}, {OHTTP: bytes.Repeat([]byte{1}, ResolutionEnvelopeCapacity+1)}} {
		if _, err := EncodeResolutionRelayEnvelope(value); err == nil {
			t.Fatal("resolution relay accepted invalid opaque capacity")
		}
	}
}

func TestResolutionRelayIOSeparatesControlAndOpaqueRecords(t *testing.T) {
	setup := resolutionRelaySetupFixture()
	var wire bytes.Buffer
	if err := WriteResolutionRelaySetup(&wire, setup); err != nil {
		t.Fatal(err)
	}
	if err := WriteResolutionRelayEnvelope(&wire, ResolutionRelayEnvelope{OHTTP: []byte{4, 5, 6}}); err != nil {
		t.Fatal(err)
	}
	gotSetup, err := ReadResolutionRelaySetup(&wire)
	if err != nil || gotSetup != setup {
		t.Fatalf("ResolutionRelaySetup IO = %+v, %v", gotSetup, err)
	}
	envelope, err := ReadResolutionRelayEnvelope(&wire)
	if err != nil || !bytes.Equal(envelope.OHTTP, []byte{4, 5, 6}) {
		t.Fatalf("ResolutionRelayEnvelope IO = %x, %v", envelope.OHTTP, err)
	}
}

func resolutionRelaySetupFixture() ResolutionRelaySetup {
	return ResolutionRelaySetup{NetworkID: identifier(41), Digest: identifier(42), AttachmentID: identifier(43),
		InitiatorNodeID: identifier(44), GatewayNodeID: identifier(45), GatewayNodePublicKey: identifier(46), Epoch: 47,
		NotAfter: time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC), EnvelopeCapacity: ResolutionEnvelopeCapacity}
}
