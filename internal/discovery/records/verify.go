package records

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

var recordDomain = []byte("ardents:discovery-record:v1\x00")

func Canonical(record Record) ([]byte, error) {
	raw, err := json.Marshal(struct {
		Version   uint32        `json:"version"`
		Node      *NodeFacts    `json:"node,omitempty"`
		Service   *ServiceFacts `json:"service,omitempty"`
		IssuedAt  time.Time     `json:"issued_at"`
		ExpiresAt time.Time     `json:"expires_at"`
	}{Version: record.Version, Node: record.Node, Service: record.Service, IssuedAt: record.IssuedAt, ExpiresAt: record.ExpiresAt})
	if err != nil {
		return nil, err
	}
	return append(append([]byte(nil), recordDomain...), raw...), nil
}

func Validate(record Record) error {
	return ValidateAt(record, time.Now().UTC())
}

func ValidateAt(record Record, now time.Time) error {
	if err := validateSignedRecord(record); err != nil {
		return err
	}
	if now.Before(record.IssuedAt) {
		return errors.New("record is not yet valid")
	}
	if !record.ExpiresAt.After(now) {
		return errors.New("record expired")
	}
	return nil
}

// ValidateRetained verifies durable authority without rejecting a record only
// because it expired while the Node was stopped.
func ValidateRetained(record Record) error {
	return validateSignedRecord(record)
}

// VerifyRetained verifies durable record authority once and returns evidence
// suitable for reuse by projections in the same process. Persisted evidence is
// validated for binding on load, but the signature is still reverified there.
func VerifyRetained(record Record, trustGeneration string, trusted bool) (VerificationEvidence, error) {
	if trustGeneration == "" {
		return VerificationEvidence{}, errors.New("trust generation is required")
	}
	if err := validateSignedRecord(record); err != nil {
		return VerificationEvidence{}, err
	}
	return evidenceFor(record, trustGeneration, trusted)
}

// ValidateEvidence checks that retained evidence describes these exact signed
// bytes and a concrete trust generation. It intentionally does not verify the
// signature; callers loading persisted state must call VerifyRetained as well.
func ValidateEvidence(record Record, evidence VerificationEvidence) error {
	if evidence.Version != EvidenceVersion {
		return errors.New("record verification evidence version is unsupported")
	}
	if generation, err := hex.DecodeString(evidence.TrustGeneration); err != nil || len(generation) != sha256.Size || hex.EncodeToString(generation) != evidence.TrustGeneration {
		return errors.New("record verification evidence trust generation is invalid")
	}
	want, err := evidenceFor(record, evidence.TrustGeneration, evidence.Trusted)
	if err != nil {
		return err
	}
	if evidence.CanonicalDigest != want.CanonicalDigest ||
		evidence.SignatureDigest != want.SignatureDigest ||
		!evidence.Signer.Equal(want.Signer) {
		return errors.New("record verification evidence does not match record")
	}
	return nil
}

func evidenceFor(record Record, trustGeneration string, trusted bool) (VerificationEvidence, error) {
	canonicalDigest, signatureDigest, signer, err := Fingerprint(record)
	if err != nil {
		return VerificationEvidence{}, err
	}
	return VerificationEvidence{
		Version:         EvidenceVersion,
		CanonicalDigest: canonicalDigest,
		SignatureDigest: signatureDigest,
		Signer:          signer,
		TrustGeneration: trustGeneration,
		Trusted:         trusted,
	}, nil
}

// Fingerprint returns a non-authoritative binding for cache lookup. It does not
// validate facts or verify the signature; only VerifyRetained creates trusted
// verification evidence.
func Fingerprint(record Record) (string, string, identityprincipal.ID, error) {
	canonical, err := Canonical(record)
	if err != nil {
		return "", "", identityprincipal.ID{}, err
	}
	signature, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil || base64.StdEncoding.EncodeToString(signature) != record.Signature || len(signature) != ed25519.SignatureSize {
		return "", "", identityprincipal.ID{}, errors.New("record signature is invalid")
	}
	canonicalDigest := sha256.Sum256(canonical)
	signatureDigest := sha256.Sum256(signature)
	return base64.RawURLEncoding.EncodeToString(canonicalDigest[:]),
		base64.RawURLEncoding.EncodeToString(signatureDigest[:]), recordSigner(record), nil
}

func recordSigner(record Record) identityprincipal.ID {
	if record.Node != nil {
		return record.Node.Principal
	}
	if record.Service != nil {
		return record.Service.NodePrincipal
	}
	return identityprincipal.ID{}
}

func validateSignedRecord(record Record) error {
	if record.Version != Version {
		return errors.New("record version is unsupported")
	}
	if (record.Node == nil) == (record.Service == nil) {
		return errors.New("record must contain exactly one facts body")
	}
	if record.IssuedAt.IsZero() || record.ExpiresAt.IsZero() || !record.ExpiresAt.After(record.IssuedAt) {
		return errors.New("record validity interval is invalid")
	}
	if record.Signature == "" {
		return errors.New("record signature is required")
	}
	if err := validateFacts(record); err != nil {
		return err
	}
	publicKey, signature, err := decodeSignatureInputs(record)
	if err != nil {
		return err
	}
	payload, err := Canonical(record)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("record signature verification failed")
	}
	return nil
}

func validateFacts(record Record) error {
	if record.Node != nil {
		if record.Node.Principal.String() == "" {
			return errors.New("node Principal is required")
		}
		if err := validateAuthority(record.Node.Principal, record.Node.PublicKey); err != nil {
			return err
		}
		return validateEndpoints(record.Node.Endpoints)
	}
	facts := record.Service
	if !validResourcePart(string(facts.ID)) || !validResourcePart(facts.Type) || !validResourcePart(string(facts.Workload)) || !validResourcePart(facts.Mode) {
		return errors.New("service discovery facts are invalid")
	}
	if facts.NodePrincipal.String() == "" {
		return errors.New("service Node Principal is required")
	}
	if err := validateAuthority(facts.NodePrincipal, facts.PublicKey); err != nil {
		return err
	}
	return validateEndpoints(facts.Endpoints)
}

func validateAuthority(principal identityprincipal.ID, encodedPublic string) error {
	expected, err := identityprincipal.FromPublicKey(encodedPublic)
	if err != nil {
		return err
	}
	if expected != principal.String() {
		return errors.New("record Node Principal does not match signing identity")
	}
	return nil
}

func validateEndpoints(endpoints []string) error {
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		if !validResourcePart(endpoint) {
			return errors.New("record endpoint is invalid")
		}
		if _, duplicate := seen[endpoint]; duplicate {
			return errors.New("record endpoint is duplicated")
		}
		seen[endpoint] = struct{}{}
	}
	return nil
}

func decodeSignatureInputs(record Record) (ed25519.PublicKey, []byte, error) {
	encodedPublic := record.PublicKeyText()
	publicKey, err := base64.StdEncoding.DecodeString(encodedPublic)
	if err != nil || base64.StdEncoding.EncodeToString(publicKey) != encodedPublic {
		return nil, nil, errors.New("record public key is invalid")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, nil, errors.New("record public key length is invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil || base64.StdEncoding.EncodeToString(signature) != record.Signature {
		return nil, nil, errors.New("record signature is invalid")
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, nil, errors.New("record signature length is invalid")
	}
	return ed25519.PublicKey(publicKey), signature, nil
}
