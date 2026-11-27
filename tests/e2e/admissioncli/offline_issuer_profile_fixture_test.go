package admissioncli_test

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"math/big"
	"testing"
	"time"
)

// Independently encode the public grammar. The synthetic RSA public moduli
// have no private counterparts and confer no token signing or runtime right.
func signedPublicIssuerProfile(t testing.TB, network, node [32]byte, signer ed25519.PrivateKey, start time.Time) []byte {
	t.Helper()
	type algorithm struct {
		OID    asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	type parameters struct {
		Hash algorithm `asn1:"explicit,tag:0"`
		Mask algorithm `asn1:"explicit,tag:1"`
		Salt int       `asn1:"explicit,tag:2"`
	}
	type subject struct {
		Algorithm algorithm
		Bits      asn1.BitString
	}
	encode := func(value any) []byte {
		raw, err := asn1.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, asn1.RawValue{FullBytes: []byte{5, 0}}}
	params := encode(parameters{hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: encode(hash)}}, 48})
	raw := append([]byte("ARDCIP01"), network[:]...)
	raw = append(raw, node[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(start.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(start.Add(time.Hour).Unix()))
	raw = binary.BigEndian.AppendUint16(raw, 3)
	for class := byte(1); class <= 3; class++ {
		n := new(big.Int).Lsh(big.NewInt(1), 2047)
		n.Add(n, big.NewInt(int64(class)*2+1))
		public := x509.MarshalPKCS1PublicKey(&rsa.PublicKey{N: n, E: 65537})
		spki := encode(subject{algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: params}}, asn1.BitString{Bytes: public, BitLength: len(public) * 8}})
		raw = binary.BigEndian.AppendUint64(raw, uint64(start.Unix()))
		raw = append(raw, class)
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(spki)))
		raw = append(raw, spki...)
	}
	return append(raw, ed25519.Sign(signer, append([]byte("ardents-closed-issuer-keys-v1\x00"), raw...))...)
}
