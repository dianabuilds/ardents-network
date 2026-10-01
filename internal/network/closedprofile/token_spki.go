package closedprofile

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
)

var (
	pssMGF1      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	pssAlgorithm = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	pssSHA384    = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	asn1Null     = []byte{0x05, 0x00}
)

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type pssParameters struct {
	HashAlgorithm    algorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm algorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                 `asn1:"explicit,tag:2"`
}

type subjectPublicKeyInfo struct {
	Algorithm        algorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// ValidateTokenSPKI accepts only the exact RSA-PSS SubjectPublicKeyInfo
// selected for closed-profile token keys.
func ValidateTokenSPKI(encoded []byte) bool {
	if len(encoded) != 346 {
		return false
	}
	var subjectPublicKeyInfo subjectPublicKeyInfo
	rest, err := asn1.Unmarshal(encoded, &subjectPublicKeyInfo)
	if err != nil || len(rest) != 0 || !subjectPublicKeyInfo.Algorithm.Algorithm.Equal(pssAlgorithm) || subjectPublicKeyInfo.SubjectPublicKey.BitLength%8 != 0 {
		return false
	}
	var parameters pssParameters
	rest, err = asn1.Unmarshal(subjectPublicKeyInfo.Algorithm.Parameters.FullBytes, &parameters)
	if err != nil || len(rest) != 0 || !parameters.HashAlgorithm.Algorithm.Equal(pssSHA384) ||
		!bytes.Equal(parameters.HashAlgorithm.Parameters.FullBytes, asn1Null) || !parameters.MaskGenAlgorithm.Algorithm.Equal(pssMGF1) || parameters.SaltLength != 48 {
		return false
	}
	var mgfHash algorithmIdentifier
	rest, err = asn1.Unmarshal(parameters.MaskGenAlgorithm.Parameters.FullBytes, &mgfHash)
	if err != nil || len(rest) != 0 || !mgfHash.Algorithm.Equal(pssSHA384) || !bytes.Equal(mgfHash.Parameters.FullBytes, asn1Null) {
		return false
	}
	publicKey, err := x509.ParsePKCS1PublicKey(subjectPublicKeyInfo.SubjectPublicKey.Bytes)
	return err == nil && publicKey.N.BitLen() == 2048 && publicKey.E == 65537 && publicKey.N.Sign() > 0 && publicKey.N.Bit(0) == 1 && (publicKey.N.BitLen()+7)/8 == 256
}
