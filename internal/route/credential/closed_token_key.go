package credential

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

var (
	closedTokenMGF1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	closedTokenRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	closedTokenSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	closedTokenNull   = []byte{0x05, 0x00}
)

type closedTokenAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type closedTokenPSSParameters struct {
	HashAlgorithm    closedTokenAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm closedTokenAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                            `asn1:"explicit,tag:2"`
}

type closedTokenSPKI struct {
	Algorithm        closedTokenAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func parseClosedTokenPublicKey(spki []byte) (*rsa.PublicKey, error) {
	if !state.ValidateClosedTokenSPKI(spki) {
		return nil, errors.New("closed token SPKI is not State-admitted")
	}
	var subject closedTokenSPKI
	rest, err := asn1.Unmarshal(spki, &subject)
	if err != nil || len(rest) != 0 || !subject.Algorithm.Algorithm.Equal(closedTokenRSAPSS) {
		return nil, errors.New("closed token SPKI algorithm is invalid")
	}
	var parameters closedTokenPSSParameters
	rest, err = asn1.Unmarshal(subject.Algorithm.Parameters.FullBytes, &parameters)
	if err != nil || len(rest) != 0 || !parameters.HashAlgorithm.Algorithm.Equal(closedTokenSHA384) ||
		!bytes.Equal(parameters.HashAlgorithm.Parameters.FullBytes, closedTokenNull) || !parameters.MaskGenAlgorithm.Algorithm.Equal(closedTokenMGF1) ||
		parameters.SaltLength != crypto.SHA384.Size() {
		return nil, errors.New("closed token SPKI parameters are invalid")
	}
	var maskHash closedTokenAlgorithmIdentifier
	rest, err = asn1.Unmarshal(parameters.MaskGenAlgorithm.Parameters.FullBytes, &maskHash)
	if err != nil || len(rest) != 0 || !maskHash.Algorithm.Equal(closedTokenSHA384) ||
		!bytes.Equal(maskHash.Parameters.FullBytes, closedTokenNull) || subject.SubjectPublicKey.BitLength%8 != 0 {
		return nil, errors.New("closed token SPKI mask parameters are invalid")
	}
	public, err := x509.ParsePKCS1PublicKey(subject.SubjectPublicKey.Bytes)
	if err != nil || public.N == nil || public.N.BitLen() != 2048 || public.E != 65537 {
		return nil, errors.New("closed token RSA public key is invalid")
	}
	return public, nil
}
