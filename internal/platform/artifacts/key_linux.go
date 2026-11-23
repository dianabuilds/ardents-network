package artifacts

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
)

type ed25519PrivateKeyInfo struct {
	Version    int
	Algorithm  pkix.AlgorithmIdentifier
	PrivateKey []byte
}

// EncodeEd25519Key returns canonical PKCS#8 PEM for the caller to erase after
// joined use. Temporary seed and ASN.1 buffers are cleared here.
func EncodeEd25519Key(key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, ErrInput
	}
	seed := key.Seed()
	defer clear(seed)
	inner, err := asn1.Marshal(seed)
	defer clear(inner)
	if err != nil {
		return nil, err
	}
	der, err := asn1.Marshal(ed25519PrivateKeyInfo{Algorithm: pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{1, 3, 101, 112}}, PrivateKey: inner})
	defer clear(der)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParseEd25519Key admits one complete canonical Ed25519 PKCS#8 structure.
// The caller owns the returned bytes and independently checks key purpose.
func ParseEd25519Key(raw []byte) (key ed25519.PrivateKey, err error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, ErrInput
	}
	defer clear(block.Bytes)
	canonical := pem.EncodeToMemory(block)
	defer clear(canonical)
	if len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || !bytes.Equal(canonical, raw) {
		return nil, ErrInput
	}
	// ParsePKCS8PrivateKey deliberately ignores ASN.1 remainder and accepts
	// other algorithms. Admit one complete canonical Ed25519 structure first:
	// unsupported private objects must never be constructed merely to reject them.
	var encoding ed25519PrivateKeyInfo
	remainder, decodeErr := asn1.Unmarshal(block.Bytes, &encoding)
	defer clear(encoding.PrivateKey)
	if decodeErr != nil || len(remainder) != 0 || encoding.Version != 0 || !encoding.Algorithm.Algorithm.Equal(asn1.ObjectIdentifier{1, 3, 101, 112}) || len(encoding.Algorithm.Parameters.FullBytes) != 0 {
		return nil, ErrInput
	}
	canonicalDER, marshalErr := asn1.Marshal(encoding)
	defer clear(canonicalDER)
	if marshalErr != nil || !bytes.Equal(canonicalDER, block.Bytes) {
		return nil, ErrInput
	}
	var seed []byte
	remainder, decodeErr = asn1.Unmarshal(encoding.PrivateKey, &seed)
	defer clear(seed)
	if decodeErr != nil || len(remainder) != 0 || len(seed) != ed25519.SeedSize {
		return nil, ErrInput
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		clear(key)
		return nil, ErrInput
	}
	// Ed25519's cached public half must agree with its seed. Never admit an
	// inconsistent private key simply because Public() exposes its cached half.
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	defer clear(derived)
	if !bytes.Equal(derived, key) {
		clear(key)
		return nil, ErrInput
	}
	return key, nil
}
