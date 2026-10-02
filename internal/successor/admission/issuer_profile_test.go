package admission

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"
)

func independentIssuerProfile(b IssuerProfileBinding, keys []TokenKey, key ed25519.PrivateKey) []byte {
	raw := append([]byte("ARDCIP01"), b.Network[:]...)
	raw = append(raw, b.Issuer[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(b.Start.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(b.End.Unix()))
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(keys)))
	for _, k := range keys {
		raw = binary.BigEndian.AppendUint64(raw, k.Window)
		raw = append(raw, k.Class)
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(k.SPKI)))
		raw = append(raw, k.SPKI...)
	}
	return append(raw, ed25519.Sign(key, append([]byte("ardents-closed-issuer-keys-v1\x00"), raw...))...)
}
func profileFixture(t testing.TB, hours int) (IssuerProfileBinding, []TokenKey, ed25519.PrivateKey) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, 32))
	start := time.Unix(3600, 0).UTC()
	b := IssuerProfileBinding{Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte(key.Public().(ed25519.PublicKey)), Start: start, End: start.Add(time.Duration(hours) * time.Hour)}
	var keys []TokenKey
	for i := 0; i < hours*3; i++ {
		keys = append(keys, TokenKey{Window: 3600 + uint64(i/3)*3600, Class: uint8(i%3 + 1), SPKI: fixtureSPKI(t, uint8(i+1))})
	}
	return b, keys, key
}
func TestIssuerProfileIndependentBytesAndCopies(t *testing.T) {
	for _, hours := range []int{1, 6} {
		b, keys, key := profileFixture(t, hours)
		request, e := PrepareIssuerProfile(b, keys)
		if e != nil {
			t.Fatal(e)
		}
		_, transcript, ok := request.Snapshot()
		if !ok {
			t.Fatal("request")
		}
		raw, e := request.Complete(ed25519.Sign(key, transcript))
		expected := independentIssuerProfile(b, keys, key)
		if e != nil || !bytes.Equal(raw, expected) || !ed25519.Verify(b.Signer[:], append([]byte("ardents-closed-issuer-keys-v1\x00"), raw[:len(raw)-64]...), raw[len(raw)-64:]) {
			t.Fatal("independent profile", e)
		}
		verified, e := VerifyIssuerProfile(raw, b)
		if e != nil {
			t.Fatal(e)
		}
		_, owned, _ := verified.Snapshot()
		owned[0].SPKI[0] ^= 1
		keys[0].SPKI[0] ^= 1
		transcript[0] ^= 1
		_, again, _ := request.Snapshot()
		if again[0] == transcript[0] {
			t.Fatal("mutable transcript")
		}
		_, owned2, _ := verified.Snapshot()
		if owned2[0].SPKI[0] == owned[0].SPKI[0] {
			t.Fatal("mutable keys")
		}
		base := LedgerBinding{Network: b.Network, Issuer: b.Issuer, Authority: [32]byte{4}, Profile: [32]byte{5}, Duty: 9, Start: b.Start, End: b.End}
		bound, e := PrepareLedgerBinding(base, verified)
		if e != nil || bound.Profile != base.Profile || bound.Authority != base.Authority || bound.Duty != base.Duty {
			t.Fatal("State facts replaced", e)
		}
		if _, e = PrepareLedgerBinding(bound, verified); e == nil {
			t.Fatal("preexisting keys replaced")
		}
	}
	if _, e := (IssuerProfileRequest{}).Complete(make([]byte, 64)); e == nil {
		t.Fatal("zero request")
	}
	if _, e := PrepareLedgerBinding(LedgerBinding{}, VerifiedIssuerProfile{}); e == nil {
		t.Fatal("zero verification")
	}
}
func TestIssuerProfileRejectsIndependentlySignedInvalidBodies(t *testing.T) {
	b, keys, key := profileFixture(t, 2)
	for _, name := range []string{"missing", "reorder", "class", "reuse", "spki", "negative", "overflow", "zero-network", "window"} {
		t.Run(name, func(t *testing.T) {
			bad := cloneBinding(b.ledger(keys)).Keys
			binding := b
			switch name {
			case "missing":
				bad = bad[:3]
			case "reorder":
				bad[0], bad[1] = bad[1], bad[0]
			case "class":
				bad[2].Class = 2
			case "reuse":
				bad[3].SPKI = bad[0].SPKI
			case "spki":
				bad[0].SPKI[0] ^= 1
			case "negative":
				binding.Start = time.Unix(-3600, 0).UTC()
			case "overflow":
				binding.End = time.Unix(1<<63-1, 0).UTC()
			case "zero-network":
				binding.Network = [32]byte{}
			case "window":
				bad[0].Window = 7200
			}
			raw := independentIssuerProfile(binding, bad, key)
			if _, e := VerifyIssuerProfile(raw, binding); e == nil {
				t.Fatal("signed bad profile accepted")
			}
			if _, e := PrepareIssuerProfile(binding, bad); e == nil {
				t.Fatal("bad encoder accepted")
			}
		})
	}
	raw := independentIssuerProfile(b, keys, key)
	for _, change := range []func([]byte){func(x []byte) { x[0] ^= 1 }, func(x []byte) { x[len(x)-1] ^= 1 }, func(x []byte) {
		binary.BigEndian.PutUint64(x[72:80], 1<<63)
		copy(x[len(x)-64:], ed25519.Sign(key, append([]byte("ardents-closed-issuer-keys-v1\x00"), x[:len(x)-64]...)))
	}} {
		bad := append([]byte(nil), raw...)
		change(bad)
		if _, e := VerifyIssuerProfile(bad, b); e == nil {
			t.Fatal("mutation accepted")
		}
	}
	for _, bad := range []IssuerProfileBinding{{Network: b.Network, Issuer: b.Issuer, Signer: [32]byte{1}, Start: b.Start, End: b.End}, {Network: [32]byte{3}, Issuer: b.Issuer, Signer: b.Signer, Start: b.Start, End: b.End}, {Network: b.Network, Issuer: [32]byte{3}, Signer: b.Signer, Start: b.Start, End: b.End}} {
		if _, e := VerifyIssuerProfile(raw, bad); e == nil {
			t.Fatal("wrong pin accepted")
		}
	}
	if _, e := VerifyIssuerProfile(append(raw, 0), b); e == nil {
		t.Fatal("extra bytes")
	}
}
func FuzzIssuerProfile(f *testing.F) {
	b, keys, key := profileFixture(f, 1)
	f.Add(independentIssuerProfile(b, keys, key))
	f.Add([]byte("ARDCIP01"))
	f.Add(make([]byte, 154))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaximumIssuerProfile+1 {
			return
		}
		v, e := VerifyIssuerProfile(raw, b)
		if e == nil {
			bound, keys, ok := v.Snapshot()
			if !ok || !bound.matches(b) || !bound.ledger(keys).valid() {
				t.Fatal("invalid verified profile")
			}
		}
	})
}
