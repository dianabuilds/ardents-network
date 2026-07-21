// Package capability owns capability assertion validation and attenuation.
// It does not own local RPC method authorization.
package capability

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"time"

	identityapi "ardents/internal/identity"
	identityprincipal "ardents/internal/identity/principal"
)

const deliveryAttestationDomain = "ardents-capability-delivery-attestation/1"
const maximumAttestationLifetime = 30 * 24 * time.Hour

func (s *Service) AttestDeliveryPublicKey(identityPrivate ed25519.PrivateKey, notAfter time.Time) (identityapi.CapabilityDeliveryAttestation, error) {
	if len(identityPrivate) != ed25519.PrivateKeySize {
		return identityapi.CapabilityDeliveryAttestation{}, fmt.Errorf("identity private key is invalid")
	}
	public := identityPrivate.Public().(ed25519.PublicKey)
	if identityprincipal.DeriveID("p", public) != s.localPrincipal {
		return identityapi.CapabilityDeliveryAttestation{}, fmt.Errorf("identity key does not match local capability principal")
	}
	now := s.clock().UTC().Truncate(time.Second)
	notAfter = notAfter.UTC().Truncate(time.Second)
	attestation := identityapi.CapabilityDeliveryAttestation{
		Version: 1, SubjectPrincipal: s.localPrincipal,
		IdentityPublicKey: append([]byte(nil), public...),
		NotBefore:         now, NotAfter: notAfter,
	}
	deliveryPublic, err := s.EnsureDeliveryPublicKey()
	if err != nil {
		return identityapi.CapabilityDeliveryAttestation{}, err
	}
	attestation.DeliveryPublicKey = deliveryPublic
	if err := validateAttestationFields(attestation, now); err != nil {
		return identityapi.CapabilityDeliveryAttestation{}, err
	}
	digest, err := attestationDigest(attestation)
	if err != nil {
		return identityapi.CapabilityDeliveryAttestation{}, err
	}
	attestation.Signature = ed25519.Sign(identityPrivate, digest)
	return attestation, nil
}

func VerifyDeliveryAttestation(attestation identityapi.CapabilityDeliveryAttestation, at time.Time) error {
	at = at.UTC().Truncate(time.Second)
	if err := validateAttestationFields(attestation, at); err != nil {
		return err
	}
	public := ed25519.PublicKey(attestation.IdentityPublicKey)
	if identityprincipal.DeriveID("p", public) != attestation.SubjectPrincipal {
		return fmt.Errorf("delivery attestation identity does not match subject")
	}
	digest, err := attestationDigest(attestation)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, digest, attestation.Signature) {
		return fmt.Errorf("delivery attestation signature is invalid")
	}
	return nil
}

func validateAttestationFields(attestation identityapi.CapabilityDeliveryAttestation, at time.Time) error {
	if attestation.Version != 1 || !validPrincipal(attestation.SubjectPrincipal) {
		return fmt.Errorf("delivery attestation version or subject is invalid")
	}
	if len(attestation.IdentityPublicKey) != ed25519.PublicKeySize || len(attestation.DeliveryPublicKey) != 32 {
		return fmt.Errorf("delivery attestation public key is invalid")
	}
	if attestation.NotBefore.Nanosecond() != 0 || attestation.NotAfter.Nanosecond() != 0 ||
		!attestation.NotBefore.Before(attestation.NotAfter) ||
		attestation.NotAfter.Sub(attestation.NotBefore) > maximumAttestationLifetime {
		return fmt.Errorf("delivery attestation validity is invalid")
	}
	if at.Before(attestation.NotBefore) || !at.Before(attestation.NotAfter) {
		return fmt.Errorf("delivery attestation is not currently valid")
	}
	return nil
}

func attestationDigest(attestation identityapi.CapabilityDeliveryAttestation) ([]byte, error) {
	var raw bytes.Buffer
	writeUint32(&raw, attestation.Version)
	if err := writeString(&raw, attestation.SubjectPrincipal); err != nil {
		return nil, err
	}
	if err := writeBytes(&raw, attestation.IdentityPublicKey); err != nil {
		return nil, err
	}
	if err := writeBytes(&raw, attestation.DeliveryPublicKey); err != nil {
		return nil, err
	}
	writeInt64(&raw, attestation.NotBefore.Unix())
	writeInt64(&raw, attestation.NotAfter.Unix())
	sum := sha256.Sum256(append(append([]byte(deliveryAttestationDomain), 0), raw.Bytes()...))
	return sum[:], nil
}

func writeBytes(out *bytes.Buffer, raw []byte) error {
	return writeString(out, string(raw))
}
