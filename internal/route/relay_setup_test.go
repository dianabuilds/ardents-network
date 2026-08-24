package route

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestRelaySetupRoundTripAndExactReady(t *testing.T) {
	setup := relaySetupFixture()
	raw, err := EncodeRelaySetup(setup)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76310000f20001041c617264656e74732d696e7465726163746976652d726f7574652d76310100000000000000000000000000000000000000000000000000000000000000000000000000000702000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000010304000000000000000000000000000000000000000000000000000000000000000500000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000684ee180"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical RelaySetup = %x, want %s", raw, want)
	}
	decoded, err := DecodeRelaySetup(raw)
	if err != nil || decoded != setup {
		t.Fatalf("decoded setup = %+v, %v", decoded, err)
	}
	readyRaw, err := EncodeRelayReady(RelayReady{Setup: setup})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := DecodeRelayReady(readyRaw)
	if err != nil || ready.Setup != setup {
		t.Fatalf("decoded ready = %+v, %v", ready, err)
	}
	if err := setup.VerifyRelayReady(ready); err != nil {
		t.Fatal(err)
	}
	ready.Setup.NextNodeID[0] ^= 1
	if err := setup.VerifyRelayReady(ready); err == nil {
		t.Fatal("substituted RelayReady was accepted")
	}
}

func TestRelaySetupRejectsMalformedAndWrongRoles(t *testing.T) {
	setup := relaySetupFixture()
	raw, err := EncodeRelaySetup(setup)
	if err != nil {
		t.Fatal(err)
	}
	badRole := append([]byte(nil), raw...)
	badRole[len(routeWireMagic)+2+2+1+1+len(Profile)+32+8+32+32] = IntroductionRole
	for index, input := range [][]byte{nil, raw[:len(raw)-1], append(raw, 0), badRole} {
		if _, err := DecodeRelaySetup(input); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
	if _, err := ReadRelaySetup(bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
}

func relaySetupFixture() RelaySetup {
	return RelaySetup{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: identifier(3), Epoch: 7,
		TransitRole: InitiatorRole, NextRole: RendezvousRole, TransitNodeID: identifier(4), NextNodeID: identifier(5),
		NextNodePublicKey: identifier(6), NotAfter: time.Unix(1_750_000_000, 0).UTC()}
}
