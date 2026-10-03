package quota

import "github.com/dianabuilds/ardents-network/internal/successor/admission"

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

// Independent fixture encoding follows the documented grammar; no production
// batch/permission/SPKI encoder is used by this oracle. Moduli are public-only
// synthetic test inputs, not keys used for signing or a qualification claim.
func batchFixture(t *testing.T, class uint8, count uint16, max uint32, id byte) ([]byte, admission.Facts, LedgerBinding) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	seed := make([]byte, 32)
	seed[0] = 1
	holder := ed25519.NewKeyFromSeed(seed)
	f := admission.Facts{Network: [32]byte{1}, Issuer: [32]byte{2}, Duty: 7, DutyNotBefore: time.Unix(3600, 0).UTC(), DutyNotAfter: time.Unix(7200, 0).UTC(), Now: time.Unix(3600, 0).UTC(), Class: class, Count: uint32(count)}
	copy(f.Authority[:], authority.Public().(ed25519.PublicKey))
	copy(f.Holder[:], holder.Public().(ed25519.PublicKey))
	binding := LedgerBinding{Network: f.Network, Issuer: f.Issuer, Authority: f.Authority, Profile: [32]byte{9}, Duty: f.Duty, Start: f.DutyNotBefore, End: f.DutyNotAfter}
	for c := uint8(1); c <= 3; c++ {
		binding.Keys = append(binding.Keys, issuerprofile.Key{Window: 3600, Class: c, SPKI: fixtureSPKI(t, c)})
	}
	p := make([]byte, 228)
	copy(p[:32], f.Network[:])
	copy(p[32:64], f.Issuer[:])
	binary.BigEndian.PutUint64(p[64:72], 7)
	p[72] = 4
	copy(p[104:136], f.Holder[:])
	binary.BigEndian.PutUint64(p[136:144], 3600)
	binary.BigEndian.PutUint64(p[144:152], 7200)
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint32(p[152+i*4:156+i*4], max)
	}
	copy(p[164:], ed25519.Sign(authority, append([]byte("ardents-issuance-permission-v1\x00"), p[:164]...)))
	request := [32]byte{id}
	spki := binding.Keys[class-1].SPKI
	keyID := sha256.Sum256(spki)
	elements := make([]byte, int(count)*259)
	for i := 0; i < int(count); i++ {
		element := elements[i*259 : (i+1)*259]
		binary.BigEndian.PutUint16(element[:2], 2)
		element[2] = keyID[31]
		element[258] = byte(i + 1)
	}
	raw := append([]byte("ARDIBR01"), p...)
	raw = append(raw, request[:]...)
	raw = append(raw, class)
	raw = binary.BigEndian.AppendUint64(raw, 3600)
	raw = append(raw, spki...)
	raw = binary.BigEndian.AppendUint16(raw, count)
	raw = append(raw, elements...)
	transcript := append([]byte("ardents-issuance-request-v1\x00"), p[72:104]...)
	transcript = append(transcript, request[:]...)
	transcript = append(transcript, class)
	transcript = binary.BigEndian.AppendUint64(transcript, 3600)
	transcript = binary.BigEndian.AppendUint16(transcript, count)
	sum := sha256.Sum256(elements)
	transcript = append(transcript, sum[:]...)
	raw = append(raw, ed25519.Sign(holder, transcript)...)
	return raw, f, binding
}

func fixtureSPKI(t testing.TB, id uint8) []byte {
	t.Helper()
	type algorithm struct {
		OID    asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	type params struct {
		Hash algorithm `asn1:"explicit,tag:0"`
		MGF  algorithm `asn1:"explicit,tag:1"`
		Salt int       `asn1:"explicit,tag:2"`
	}
	type subject struct {
		Algorithm algorithm
		Bits      asn1.BitString
	}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, asn1.RawValue{FullBytes: []byte{5, 0}}}
	hashRaw, e := asn1.Marshal(hash)
	if e != nil {
		t.Fatal(e)
	}
	parameters, e := asn1.Marshal(params{hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: hashRaw}}, 48})
	if e != nil {
		t.Fatal(e)
	}
	n := new(big.Int).Lsh(big.NewInt(1), 2047)
	n.Add(n, big.NewInt(int64(id)*2+1))
	public := rsa.PublicKey{N: n, E: 65537}
	rsaRaw := x509.MarshalPKCS1PublicKey(&public)
	raw, e := asn1.Marshal(subject{algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: parameters}}, asn1.BitString{Bytes: rsaRaw, BitLength: len(rsaRaw) * 8}})
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func TestBatchContract(t *testing.T) {
	for class := uint8(1); class <= 3; class++ {
		for _, count := range []uint16{1, 32} {
			raw, f, b := batchFixture(t, class, count, 32, 1)
			if _, got := verifyBatch(t.Context(), raw, f, b); got != admission.Accepted {
				t.Fatalf("class/count %d/%d: %s", class, count, got)
			}
		}
	}
	for _, test := range []struct {
		name   string
		change func([]byte, *admission.Facts, *LedgerBinding)
		want   admission.Outcome
	}{
		{"holder signature", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) { raw[len(raw)-1] ^= 1 }, admission.Signature},
		{"permission signature", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) { raw[172] ^= 1 }, admission.Signature},
		{"element zero", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) { raw[883] = 0 }, admission.Malformed},
		{"element out of range", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) {
			for i := 628; i < 884; i++ {
				raw[i] = 255
			}
		}, admission.Malformed},
		{"bad SPKI", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) { raw[300] ^= 1 }, admission.Malformed},
		{"key mismatch", func(_ []byte, _ *admission.Facts, b *LedgerBinding) { b.Keys[1].SPKI = fixtureSPKI(t, 9) }, admission.Binding},
		{"count mismatch", func(_ []byte, f *admission.Facts, _ *LedgerBinding) { f.Count++ }, admission.Binding},
		{"duty mismatch", func(_ []byte, f *admission.Facts, _ *LedgerBinding) { f.Duty++ }, admission.Binding},
		{"authority mismatch", func(_ []byte, f *admission.Facts, _ *LedgerBinding) { f.Authority[0]++ }, admission.Signature},
		{"expiry", func(_ []byte, f *admission.Facts, _ *LedgerBinding) { f.Now = f.DutyNotAfter }, admission.Validity},
		{"time overflow", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) {
			binary.BigEndian.PutUint64(raw[269:277], math.MaxUint64/3600*3600)
		}, admission.Malformed},
		{"zero ID", func(raw []byte, _ *admission.Facts, _ *LedgerBinding) { clear(raw[236:268]) }, admission.Malformed},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, f, b := batchFixture(t, 2, 1, 32, 1)
			test.change(raw, &f, &b)
			if _, got := verifyBatch(t.Context(), raw, f, b); got != test.want {
				t.Fatalf("got %s want %s", got, test.want)
			}
		})
	}
	raw, f, b := batchFixture(t, 2, 1, 32, 1)
	for _, size := range []int{0, 8, 883, len(raw) - 1, len(raw) + 1, 16385} {
		changed := make([]byte, size)
		copy(changed, raw)
		if _, got := verifyBatch(t.Context(), changed, f, b); got != admission.Malformed {
			t.Fatalf("size %d: %s", size, got)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, got := verifyBatch(ctx, raw, f, b); got != admission.Canceled {
		t.Fatal(got)
	}
}

func TestBindingInventory(t *testing.T) {
	_, _, b := batchFixture(t, 2, 1, 32, 1)
	if !b.valid() {
		t.Fatal("valid binding refused")
	}
	b.Keys[1].SPKI = b.Keys[0].SPKI
	if b.valid() {
		t.Fatal("cross-cohort key reuse accepted")
	}
}

func FuzzBatchMutationRequiresValidSignatures(f *testing.F) {
	f.Add(uint16(0), uint8(1))
	f.Add(uint16(883), uint8(1))
	f.Fuzz(func(t *testing.T, offset uint16, mask uint8) {
		raw, facts, binding := batchFixture(t, 2, 1, 32, 1)
		original := append([]byte(nil), raw...)
		raw[int(offset)%len(raw)] ^= mask
		_, got := verifyBatch(t.Context(), raw, facts, binding)
		if got == admission.Accepted && !bytes.Equal(raw, original) {
			t.Fatal("accepted mutated signed canonical batch")
		}
	})
}
