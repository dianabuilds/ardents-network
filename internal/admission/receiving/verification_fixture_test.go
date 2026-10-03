package receiving

import (
	"crypto"
	"crypto/rsa"
	"encoding/asn1"
	"errors"
)

// Independent test key encoding, reused from the token dependency baseline.
// No test key material or encoder is exported through the domain interface.
var (
	oidMGF1       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	oidRSAPSS     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	oidSHA384     = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	asn1NullValue = asn1.RawValue{FullBytes: []byte{0x05, 0x00}}
)

type rsaPSSAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type rsaPSSParameters struct {
	HashAlgorithm    rsaPSSAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm rsaPSSAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                       `asn1:"explicit,tag:2"`
}

type rsaPSSSubjectPublicKeyInfo struct {
	Algorithm        rsaPSSAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func encodeSelectedRSAPSSSPKI(publicKey *rsa.PublicKey) ([]byte, error) {
	if publicKey == nil || publicKey.N == nil || publicKey.N.BitLen() != 2048 || publicKey.E != 65537 {
		return nil, errors.New("selected RSA-PSS public key is invalid")
	}
	hashAlgorithm := rsaPSSAlgorithmIdentifier{Algorithm: oidSHA384, Parameters: asn1NullValue}
	hashDER, err := asn1.Marshal(hashAlgorithm)
	if err != nil {
		return nil, err
	}
	parametersDER, err := asn1.Marshal(rsaPSSParameters{
		HashAlgorithm:    hashAlgorithm,
		MaskGenAlgorithm: rsaPSSAlgorithmIdentifier{Algorithm: oidMGF1, Parameters: asn1.RawValue{FullBytes: hashDER}},
		SaltLength:       crypto.SHA384.Size(),
	})
	if err != nil {
		return nil, err
	}
	rsaDER, err := asn1.Marshal(*publicKey)
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(rsaPSSSubjectPublicKeyInfo{
		Algorithm:        rsaPSSAlgorithmIdentifier{Algorithm: oidRSAPSS, Parameters: asn1.RawValue{FullBytes: parametersDER}},
		SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8},
	})
}
