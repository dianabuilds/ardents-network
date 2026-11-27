package issuerprofile

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
)

var (
	pssOID  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	mgfOID  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	hashOID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
)

type keyAlgorithm struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}
type keyParameters struct {
	Hash keyAlgorithm `asn1:"explicit,tag:0"`
	Mask keyAlgorithm `asn1:"explicit,tag:1"`
	Salt int          `asn1:"explicit,tag:2"`
}
type keySubject struct {
	Algorithm keyAlgorithm
	Public    asn1.BitString
}

// ErrTokenSPKI is the refusal for a key outside the canonical token grammar.
var ErrTokenSPKI = errors.New("closed token SPKI is not canonical")

// ParseKey accepts only the selected canonical RSA-PSS public encoding.
// It preserves key identity rather than normalizing admitted bytes.
func ParseKey(raw []byte) (*rsa.PublicKey, error) {
	if len(raw) != 346 {
		return nil, ErrTokenSPKI
	}
	var subject keySubject
	rest, err := asn1.Unmarshal(raw, &subject)
	if err != nil || len(rest) != 0 || !subject.Algorithm.Algorithm.Equal(pssOID) || subject.Public.BitLength != len(subject.Public.Bytes)*8 {
		return nil, ErrTokenSPKI
	}
	public, err := x509.ParsePKCS1PublicKey(subject.Public.Bytes)
	if err != nil || public.N == nil || public.N.Sign() <= 0 || public.N.BitLen() != 2048 || public.N.Bit(0) != 1 || public.E != 65537 {
		return nil, ErrTokenSPKI
	}
	canonical, err := EncodeKey(public)
	if err != nil || !bytes.Equal(raw, canonical) {
		return nil, ErrTokenSPKI
	}
	return public, nil
}

// EncodeKey encodes an eligible public key in the selected canonical SPKI grammar.
func EncodeKey(public *rsa.PublicKey) ([]byte, error) {
	if public == nil || public.N == nil || public.N.Sign() <= 0 || public.N.BitLen() != 2048 || public.N.Bit(0) != 1 || public.E != 65537 {
		return nil, ErrInvalid
	}
	hash := keyAlgorithm{hashOID, asn1.RawValue{FullBytes: []byte{5, 0}}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		return nil, err
	}
	parameters, err := asn1.Marshal(keyParameters{hash, keyAlgorithm{mgfOID, asn1.RawValue{FullBytes: hashDER}}, 48})
	if err != nil {
		return nil, err
	}
	rsaDER := x509.MarshalPKCS1PublicKey(public)
	return asn1.Marshal(keySubject{keyAlgorithm{pssOID, asn1.RawValue{FullBytes: parameters}}, asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
}

// ValidateTokenSPKI shares the canonical key parser used by profile preparation
// and verification. A noncanonical equivalent key is not the same token key.
func ValidateTokenSPKI(raw []byte) bool {
	_, err := ParseKey(raw)
	return err == nil
}
