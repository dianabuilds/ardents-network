package issuance

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"testing"
	"time"
)

func testBinding(hours int) Binding {
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	return Binding{Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte{3}, Start: start, End: start.Add(time.Duration(hours) * time.Hour)}
}
func TestRealKeyInventory(t *testing.T) {
	for _, hours := range []int{1, 6} {
		b := testBinding(hours)
		raw, err := generate(t.Context(), b)
		if err != nil {
			t.Fatal(err)
		}
		v, err := decodeMaterial(raw, b)
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Keys) != hours*3 || v.Digest != sha256.Sum256(v.canonical()) {
			t.Fatal("inventory identity")
		}
		seen := map[string]bool{}
		for i, k := range v.Keys {
			if k.Window != uint64(b.Start.Unix()+int64(i/3)*3600) || k.Class != uint8(i%3+1) || len(k.SPKI) != 346 {
				t.Fatal("cohort")
			}
			key, valid := tokenKey(k.SPKI)
			if !valid || key.N.BitLen() != 2048 || key.E != 65537 || seen[string(k.SPKI)] {
				t.Fatal("key grammar/reuse")
			}
			seen[string(k.SPKI)] = true
			bad := append([]byte(nil), k.SPKI...)
			bad[len(bad)-1] ^= 1
			if _, ok := tokenKey(bad); ok {
				t.Fatal("invalid exponent accepted")
			}
		}
		clear(raw)
	}
}
func TestMaterialRejectsIndependentCorruption(t *testing.T) {
	b := testBinding(1)
	raw, err := generate(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	cases := map[string][]byte{}
	for name, offset := range map[string]int{"magic": 0, "binding": 8, "count": 121, "window": 122, "class": 130, "DER": 133} {
		v := append([]byte(nil), raw...)
		v[offset] ^= 0xff
		cases[name] = v
	}
	cases["tail"] = append(append([]byte(nil), raw...), 0)
	overflowing := append([]byte(nil), raw...)
	binary.BigEndian.PutUint64(overflowing[122:], ^uint64(0))
	cases["unsigned time overflow"] = overflowing
	cases["short"] = raw[:len(raw)-1]
	duplicate := append([]byte(nil), raw...)
	n := int(binary.BigEndian.Uint16(raw[131:133]))
	private, err := x509.ParsePKCS1PrivateKey(raw[133 : 133+n])
	if err != nil {
		t.Fatal(err)
	}
	private.E = 3
	invalidDER := x509.MarshalPKCS1PrivateKey(private)
	invalid := append([]byte(nil), raw[:131]...)
	invalid = binary.BigEndian.AppendUint16(invalid, uint16(len(invalidDER)))
	invalid = append(invalid, invalidDER...)
	invalid = append(invalid, raw[133+n:]...)
	cases["wrong exponent"] = invalid
	second := 133 + n
	secondN := int(binary.BigEndian.Uint16(raw[second+9:]))
	duplicate = append(duplicate[:second+9], binary.BigEndian.AppendUint16(nil, uint16(n))...)
	duplicate = append(duplicate, raw[133:133+n]...)
	duplicate = append(duplicate, raw[second+11+secondN:]...)
	cases["reused key"] = duplicate
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeMaterial(v, b); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	changed := b
	changed.Signer[0]++
	if _, e := decodeMaterial(raw, changed); e == nil {
		t.Fatal("rebound")
	}
	for _, bad := range []Binding{testBinding(0), testBinding(7), {Network: b.Network, Issuer: b.Issuer, Signer: b.Signer, Start: time.Unix(-3600, 0).UTC(), End: b.End}} {
		if bad.valid() {
			t.Fatal("invalid interval")
		}
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e := generate(canceled, b); e != context.Canceled {
		t.Fatal(e)
	}
}
