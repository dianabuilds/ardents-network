package capsule

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"
)

func candidateFixture() Request {
	return Request{Network: [32]byte{1}, Target: [32]byte{2}, PublicationDigest: [32]byte{3}, Revision: 7,
		RendezvousNode: [32]byte{4}, RendezvousDutyGeneration: 9, JoinSecret: [32]byte{5}, HandshakeContext: [32]byte{6},
		ProfileDigest: [32]byte{7}, ConnectionNonce: [32]byte{8}, AttachmentGeneration: 11, Deadline: time.Unix(2000000000, 0).UTC(),
		InitiatorBinding: [32]byte{9}, WorkSafetyNotAfter: 2100000000, WorkSafetyMaximum: 2200000000, NoNewRecoveryAfter: 1900000000}
}

func TestPrivateRequestIndependentCanonicalOffsets(t *testing.T) {
	request := candidateFixture()
	// Independent literal offset oracle from the accepted 344-byte grammar.
	expected := make([]byte, 344)
	for _, item := range []struct {
		offset int
		value  byte
	}{{0, 1}, {32, 2}, {64, 3}, {104, 4}, {144, 5}, {176, 6}, {208, 7}, {240, 8}, {288, 9}} {
		expected[item.offset] = item.value
	}
	for _, item := range []struct {
		offset int
		value  uint64
	}{{96, 7}, {136, 9}, {272, 11}, {280, 2000000000}, {320, 2100000000}, {328, 2200000000}, {336, 1900000000}} {
		binary.BigEndian.PutUint64(expected[item.offset:item.offset+8], item.value)
	}
	raw, err := encodeRequest(request)
	if err != nil || !bytes.Equal(raw, expected) {
		t.Fatal("canonical private offsets changed", err)
	}
	digest, err := request.Digest()
	if err != nil || digest != sha256.Sum256(expected) {
		t.Fatal("complete independent plaintext commitment changed", err)
	}
	parsed, err := ParseRequest(expected)
	if err != nil || parsed != request {
		t.Fatal("independent private vector differs", err)
	}
	for _, offset := range []int{280, 320, 328, 336} {
		mutated := append([]byte(nil), expected...)
		binary.BigEndian.PutUint64(mutated[offset:offset+8], 1<<63)
		if _, err := ParseRequest(mutated); err == nil {
			t.Fatal("time overflow accepted", offset)
		}
	}
	for _, length := range []int{0, 343, 345} {
		if _, err := ParseRequest(make([]byte, length)); err == nil {
			t.Fatal("foreign private length accepted", length)
		}
	}
}

func TestSealBindsExactV3EnvelopeHeaderAndProfile(t *testing.T) {
	request := candidateFixture()
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var public [32]byte
	copy(public[:], private.PublicKey().Bytes())
	header := Header{Slot: [32]byte{10}, DeliveryNonce: [32]byte{12}, Revision: 7, Expiry: request.Deadline}
	envelope, _, err := Seal(header, public, request)
	if err != nil {
		t.Fatal(err)
	}
	raw := envelope.Bytes()
	if len(raw) != 474 || binary.BigEndian.Uint16(raw[112:114]) != 360 || raw[0] != 10 || binary.BigEndian.Uint64(raw[32:40]) != 7 || binary.BigEndian.Uint64(raw[40:48]) != 2000000000 || raw[48] != 12 {
		t.Fatal("visible canonical envelope changed")
	}
	parsed, err := Parse(raw)
	if err != nil || parsed != envelope {
		t.Fatal("detached envelope changed", err)
	}
	info := append([]byte("ardents-introduction-capsule-v3\x00"), request.ProfileDigest[:]...)
	if !bytes.Equal(Info(request.ProfileDigest), info) || !bytes.Equal(envelope.AssociatedData(request.ProfileDigest), append(append([]byte(nil), info...), raw[:80]...)) {
		t.Fatal("info/header transcript changed")
	}
	key, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	open := func(raw []byte, info []byte) ([]byte, error) {
		r, err := hpke.NewRecipient(raw[80:112], key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
		if err != nil {
			return nil, err
		}
		return r.Open(append(append([]byte(nil), info...), raw[:80]...), raw[114:])
	}
	plain, err := open(raw, info)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(plain)
	got, err := ParseRequest(plain)
	if err != nil || got != request {
		t.Fatal("selected suite changed candidate", err)
	}
	for _, offset := range []int{0, 32, 40, 48, 80, 114, 473} {
		changed := append([]byte(nil), raw...)
		changed[offset] ^= 1
		plain, err := open(changed, info)
		clear(plain)
		if err == nil {
			t.Fatal("altered header/encapsulation/ciphertext authenticated", offset)
		}
	}
	foreign := append([]byte(nil), info...)
	foreign[len(foreign)-1] ^= 1
	plain, err = open(raw, foreign)
	clear(plain)
	if err == nil {
		t.Fatal("foreign profile authenticated")
	}
	for _, length := range []int{0, 473, 475} {
		if _, err := Parse(make([]byte, length)); err == nil {
			t.Fatal("foreign envelope size accepted", length)
		}
	}
	changed := append([]byte(nil), raw...)
	binary.BigEndian.PutUint16(changed[112:114], 359)
	if _, err := Parse(changed); err == nil {
		t.Fatal("legacy ciphertext size accepted")
	}
	raw[0] ^= 1
	if envelope.Header().Slot != header.Slot {
		t.Fatal("caller mutated retained envelope")
	}
}
