package route

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestRelaySetupSenderVectorAndExactReady(t *testing.T) {
	setup := relaySetupFixture()
	raw, err := EncodeRelaySetup(setup)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76320000f20002041c617264656e74732d696e7465726163746976652d726f7574652d76320100000000000000000000000000000000000000000000000000000000000000000000000000000702000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000010304000000000000000000000000000000000000000000000000000000000000000500000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000684ee180"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical RelaySetup = %x, want %s", raw, want)
	}
	readyRaw, err := relayEnvelope(relayReadyKind, setup)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := ReadRelayReady(bytes.NewReader(readyRaw))
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		t.Fatalf("RelayReady = %+v, %v", ready, err)
	}
	ready.Setup.NextNodeID[0] ^= 1
	if err := setup.VerifyRelayReady(ready); err == nil {
		t.Fatal("substituted RelayReady was accepted")
	}
}

func TestRelaySenderRejectsWrongRoleAndMalformedReady(t *testing.T) {
	setup := relaySetupFixture()
	setup.TransitRole = IntroductionRole
	if _, err := EncodeRelaySetup(setup); err == nil {
		t.Fatal("RelaySetup sender accepted a wrong role")
	}
	valid := relaySetupFixture()
	readyRaw, err := relayEnvelope(relayReadyKind, valid)
	if err != nil {
		t.Fatal(err)
	}
	for index, input := range [][]byte{nil, readyRaw[:len(readyRaw)-1], append(readyRaw, 0)} {
		if _, err := DecodeRelayReady(input); err == nil {
			t.Fatalf("ready mutation %d was accepted", index)
		}
	}
}

func TestRelayReadyAcceptsTheSameCanonicalSecondAcrossTimeLocations(t *testing.T) {
	setup := relaySetupFixture()
	ready := RelayReady{Setup: setup}
	ready.Setup.NotAfter = setup.NotAfter.In(time.FixedZone("fixture-offset", 3*60*60))
	if err := setup.VerifyRelayReady(ready); err != nil {
		t.Fatalf("same canonical RelayReady second was rejected: %v", err)
	}
}

func relaySetupFixture() RelaySetup {
	return RelaySetup{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: identifier(3), Epoch: 7,
		TransitRole: InitiatorRole, NextRole: RendezvousRole, TransitNodeID: identifier(4), NextNodeID: identifier(5),
		NextNodePublicKey: identifier(6), NotAfter: time.Unix(1_750_000_000, 0).UTC()}
}
