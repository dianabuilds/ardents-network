package processfixture

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
)

func TestSignedInstanceResponseBindsOriginalRequestAndCredential(t *testing.T) {
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, ed25519.SeedSize))
	defer clear(authority)
	network, instance, commitment := [32]byte{1}, [32]byte{2}, [32]byte{3}
	raw, target := SignedInstanceResponse(authority, network, instance, commitment, 7, 100, 200)
	envelope := append([]byte("ardents-service-instance-response-v3\x00"), commitment[:]...)
	if !bytes.HasPrefix(raw, envelope) || len(raw) != len(envelope)+222 {
		t.Fatalf("unexpected response envelope length or commitment: %d", len(raw))
	}
	credential := raw[len(envelope):]
	public := authority.Public().(ed25519.PublicKey)
	if !bytes.Equal(credential[:34], append([]byte{0, 3}, public...)) ||
		!bytes.Equal(credential[34:66], target[:]) || !bytes.Equal(credential[66:98], instance[:]) ||
		binary.BigEndian.Uint64(credential[98:106]) != 7 || binary.BigEndian.Uint64(credential[106:114]) != 100 ||
		binary.BigEndian.Uint64(credential[114:122]) != 200 || !bytes.Equal(credential[122:154], network[:]) ||
		binary.BigEndian.Uint32(credential[154:158]) != 3 ||
		!ed25519.Verify(public, credential[:158], credential[158:]) {
		t.Fatal("fixture credential did not sign the exact supplied fields")
	}
}
