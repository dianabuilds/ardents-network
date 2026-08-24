package route

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestIntroductionSlotRegistrationV1CanonicalVector(t *testing.T) {
	input := IntroductionSlotRegistration{Reachability: identifier(71), JoinHandle: identifier(72),
		NotAfter: time.Unix(1_750_000_000, 0).UTC()}
	raw, err := EncodeIntroductionSlotRegistration(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76310000680001071c617264656e74732d696e7465726163746976652d726f7574652d76314700000000000000000000000000000000000000000000000000000000000000480000000000000000000000000000000000000000000000000000000000000000000000684ee180"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical IntroductionSlotRegistration vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeIntroductionSlotRegistration(raw)
	if err != nil || decoded != input {
		t.Fatalf("decoded IntroductionSlotRegistration = %+v, %v", decoded, err)
	}
}

func TestIntroductionSlotRegistrationV1RefusesFractionalExpiry(t *testing.T) {
	_, err := EncodeIntroductionSlotRegistration(IntroductionSlotRegistration{Reachability: identifier(75), JoinHandle: identifier(76),
		NotAfter: time.Unix(1_750_000_000, 1).UTC()})
	if err == nil {
		t.Fatal("fractional expiry was accepted")
	}
}

func TestIntroductionSlotRegistrationV1RefusesMutation(t *testing.T) {
	raw, err := EncodeIntroductionSlotRegistration(IntroductionSlotRegistration{Reachability: identifier(73), JoinHandle: identifier(74),
		NotAfter: time.Unix(1_750_000_000, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	wrongKind := append([]byte(nil), raw...)
	wrongKind[len(routeWireMagic)+2+2] = endpointTransitBindingKind
	for index, value := range [][]byte{nil, raw[:len(raw)-1], append(raw, 0), wrongKind} {
		if _, err := DecodeIntroductionSlotRegistration(value); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
}
