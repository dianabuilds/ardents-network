package credential

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/cloudflare/circl/blindsign/blindrsa"
)

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

// TestBlindRSABaselineUsesFreshNonceAndStandardVerification keeps the selected
// dependency admission concrete without introducing its token wire or journals.
func TestBlindRSABaselineUsesFreshNonceAndStandardVerification(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate dedicated blind RSA key: %v", err)
	}
	if err := privateKey.Validate(); err != nil {
		t.Fatalf("validate dedicated blind RSA key: %v", err)
	}

	client, err := blindrsa.NewClient(blindrsa.SHA384PSSDeterministic, &privateKey.PublicKey)
	if err != nil {
		t.Fatalf("construct selected blind RSA client: %v", err)
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("draw fresh token nonce: %v", err)
	}
	input, err := client.Prepare(rand.Reader, append([]byte("ardents-admission-token\x00"), nonce...))
	if err != nil {
		t.Fatalf("prepare nonce-bearing token input: %v", err)
	}
	blinded, state, err := client.Blind(rand.Reader, input)
	if err != nil {
		t.Fatalf("blind token input: %v", err)
	}
	if got, want := len(blinded), (privateKey.PublicKey.N.BitLen()+7)/8; got != want {
		t.Fatalf("blinded request length = %d, want %d", got, want)
	}
	signer := blindrsa.NewSigner(privateKey)
	if _, err := signer.BlindSign(blinded[:len(blinded)-1]); err == nil {
		t.Fatal("selected blind RSA signer accepted a malformed blinded request length")
	}
	blindedSignature, err := signer.BlindSign(blinded)
	if err != nil {
		t.Fatalf("blind-sign token input: %v", err)
	}
	signature, err := client.Finalize(state, blindedSignature)
	if err != nil {
		t.Fatalf("finalize blind signature: %v", err)
	}
	if err := client.Verify(input, signature); err != nil {
		t.Fatalf("verify with selected CIRCL client: %v", err)
	}

	digest := sha512.Sum384(input)
	if err := rsa.VerifyPSS(&privateKey.PublicKey, crypto.SHA384, digest[:], signature,
		&rsa.PSSOptions{Hash: crypto.SHA384, SaltLength: crypto.SHA384.Size()}); err != nil {
		t.Fatalf("verify selected signature with Go standard library: %v", err)
	}

	spki, err := encodeSelectedRSAPSSSPKI(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encode selected RSA-PSS SPKI: %v", err)
	}
	if got, want := len(spki), 346; got != want {
		t.Fatalf("selected RSA-PSS SPKI length = %d, want %d", got, want)
	}
	keyID := sha256.Sum256(spki)
	parsed, err := parseSelectedRSAPSSSPKI(spki)
	if err != nil {
		t.Fatalf("parse selected RSA-PSS SPKI: %v", err)
	}
	reencoded, err := encodeSelectedRSAPSSSPKI(parsed)
	if err != nil {
		t.Fatalf("re-encode selected RSA-PSS SPKI: %v", err)
	}
	if !bytes.Equal(reencoded, spki) || sha256.Sum256(reencoded) != keyID {
		t.Fatal("selected RSA-PSS SPKI or key ID changed after parse and re-encode")
	}

	genericSPKI, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encode generic RSA SPKI: %v", err)
	}
	if bytes.Equal(genericSPKI, spki) || sha256.Sum256(genericSPKI) == keyID {
		t.Fatal("generic RSA SPKI was accepted as the selected RSA-PSS key ID")
	}
	if _, err := parseSelectedRSAPSSSPKI(genericSPKI); err == nil {
		t.Fatal("selected RSA-PSS parser accepted generic RSA SPKI")
	}
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

func parseSelectedRSAPSSSPKI(encoded []byte) (*rsa.PublicKey, error) {
	var subjectPublicKeyInfo rsaPSSSubjectPublicKeyInfo
	rest, err := asn1.Unmarshal(encoded, &subjectPublicKeyInfo)
	if err != nil || len(rest) != 0 || !subjectPublicKeyInfo.Algorithm.Algorithm.Equal(oidRSAPSS) {
		return nil, errors.New("selected RSA-PSS SPKI is invalid")
	}
	var parameters rsaPSSParameters
	rest, err = asn1.Unmarshal(subjectPublicKeyInfo.Algorithm.Parameters.FullBytes, &parameters)
	if err != nil || len(rest) != 0 || !parameters.HashAlgorithm.Algorithm.Equal(oidSHA384) ||
		!bytes.Equal(parameters.HashAlgorithm.Parameters.FullBytes, asn1NullValue.FullBytes) ||
		!parameters.MaskGenAlgorithm.Algorithm.Equal(oidMGF1) || parameters.SaltLength != crypto.SHA384.Size() {
		return nil, errors.New("selected RSA-PSS parameters are invalid")
	}
	var mgfHash rsaPSSAlgorithmIdentifier
	rest, err = asn1.Unmarshal(parameters.MaskGenAlgorithm.Parameters.FullBytes, &mgfHash)
	if err != nil || len(rest) != 0 || !mgfHash.Algorithm.Equal(oidSHA384) ||
		!bytes.Equal(mgfHash.Parameters.FullBytes, asn1NullValue.FullBytes) || subjectPublicKeyInfo.SubjectPublicKey.BitLength%8 != 0 {
		return nil, errors.New("selected RSA-PSS MGF1 parameters are invalid")
	}
	publicKey, err := x509.ParsePKCS1PublicKey(subjectPublicKeyInfo.SubjectPublicKey.Bytes)
	if err != nil || publicKey.N.BitLen() != 2048 || publicKey.E != 65537 {
		return nil, errors.New("selected RSA-PSS public key is invalid")
	}
	return publicKey, nil
}
