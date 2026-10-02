package issuerprofile

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"testing"
)

// Independent DER oracle: synthetic public moduli are never used for signing.
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

func TestPublicKeyCanonicalBytes(t *testing.T) {
	raw := fixtureSPKI(t, 1)
	key, ok := ParseKey(raw)
	if !ok {
		t.Fatal("independent key refused")
	}
	encoded, err := EncodeKey(key)
	if err != nil || !bytes.Equal(raw, encoded) {
		t.Fatal("canonical bytes changed", err)
	}
	for _, bad := range [][]byte{nil, append(append([]byte(nil), raw...), 0), raw[1:]} {
		if _, ok := ParseKey(bad); ok {
			t.Fatal("invalid DER accepted")
		}
	}
	for _, bad := range []*rsa.PublicKey{nil, {}, {N: big.NewInt(3), E: 65537}, {N: key.N, E: 3}} {
		if _, err := EncodeKey(bad); err == nil {
			t.Fatal("invalid public key encoded")
		}
	}
}
