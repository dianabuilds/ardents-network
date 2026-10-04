package network

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"fmt"
)

var (
	closedProfileMGF1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	closedProfileRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	closedProfileSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	closedProfileNull   = []byte{0x05, 0x00}
)

type closedProfileAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}
type closedProfilePSSParameters struct {
	HashAlgorithm    closedProfileAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm closedProfileAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                              `asn1:"explicit,tag:2"`
}
type closedProfileSubjectPublicKeyInfo struct {
	Algorithm        closedProfileAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func closedFixtureSPKI() ([]byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	hash := closedProfileAlgorithmIdentifier{Algorithm: closedProfileSHA384, Parameters: asn1.RawValue{FullBytes: closedProfileNull}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		return nil, err
	}
	parameters, err := asn1.Marshal(closedProfilePSSParameters{HashAlgorithm: hash,
		MaskGenAlgorithm: closedProfileAlgorithmIdentifier{Algorithm: closedProfileMGF1, Parameters: asn1.RawValue{FullBytes: hashDER}}, SaltLength: 48})
	if err != nil {
		return nil, err
	}
	rsaDER, err := asn1.Marshal(privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	spki, err := asn1.Marshal(closedProfileSubjectPublicKeyInfo{Algorithm: closedProfileAlgorithmIdentifier{Algorithm: closedProfileRSAPSS,
		Parameters: asn1.RawValue{FullBytes: parameters}}, SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
	if err != nil || len(spki) != 346 {
		return nil, fmt.Errorf("encode fixture SPKI: %d bytes, %w", len(spki), err)
	}
	return spki, nil
}
