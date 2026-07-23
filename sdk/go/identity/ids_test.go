package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestPublishedPrincipalAndDeviceIdentifierVector(t *testing.T) {
	public, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := PrincipalID(ed25519.PublicKey(public))
	if err != nil {
		t.Fatal(err)
	}
	device, err := DeviceID(ed25519.PublicKey(public))
	if err != nil {
		t.Fatal(err)
	}
	if principal != "p1_3jpjq5f6xcseuwt3lqehljybby73urlizy6lncqvx6ef67yk4tsq" {
		t.Fatalf("PrincipalID() = %q", principal)
	}
	if device != "d1_k6b6xtrqwcz6zwym6sotzgcvgioqj3kqp3ekvcvuj2xgawblhspa" {
		t.Fatalf("DeviceID() = %q", device)
	}
}

func TestPrincipalAndDeviceIdentifierRejectWrongKeyLength(t *testing.T) {
	for _, public := range []ed25519.PublicKey{
		nil,
		make(ed25519.PublicKey, ed25519.PublicKeySize-1),
		make(ed25519.PublicKey, ed25519.PublicKeySize+1),
	} {
		if _, err := PrincipalID(public); err == nil {
			t.Fatalf("PrincipalID() accepted %d-byte key", len(public))
		}
		if _, err := DeviceID(public); err == nil {
			t.Fatalf("DeviceID() accepted %d-byte key", len(public))
		}
	}
}
