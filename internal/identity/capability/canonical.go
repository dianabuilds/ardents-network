package capability

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	identityapi "ardents/internal/identity"
)

const grantSignatureDomain = "ardents-capability-grant/1"
const revocationSignatureDomain = "ardents-capability-revocation/1"

func SignGrant(grant identityapi.CapabilityGrant, private ed25519.PrivateKey) (identityapi.CapabilityGrant, error) {
	if len(private) != ed25519.PrivateKeySize {
		return identityapi.CapabilityGrant{}, fmt.Errorf("issuer private key is invalid")
	}
	return SignGrantWith(grant, func(message []byte) ([]byte, error) {
		return ed25519.Sign(private, message), nil
	})
}

// SignGrantWith signs the canonical Channel Grant digest through an external
// signer seam so an authority does not need access to private key bytes.
func SignGrantWith(grant identityapi.CapabilityGrant, sign func([]byte) ([]byte, error)) (identityapi.CapabilityGrant, error) {
	if sign == nil {
		return identityapi.CapabilityGrant{}, fmt.Errorf("issuer signer is required")
	}
	raw, err := canonicalGrant(grant)
	if err != nil {
		return identityapi.CapabilityGrant{}, err
	}
	grant.Signature, err = sign(digest(grantSignatureDomain, raw))
	if err != nil || len(grant.Signature) != ed25519.SignatureSize {
		return identityapi.CapabilityGrant{}, fmt.Errorf("issuer signing failed")
	}
	return grant, nil
}

func SignRevocation(rev identityapi.CapabilityRevocation, private ed25519.PrivateKey) (identityapi.CapabilityRevocation, error) {
	if len(private) != ed25519.PrivateKeySize {
		return identityapi.CapabilityRevocation{}, fmt.Errorf("issuer private key is invalid")
	}
	return SignRevocationWith(rev, func(message []byte) ([]byte, error) {
		return ed25519.Sign(private, message), nil
	})
}

// SignRevocationWith signs a canonical revocation through an external signer
// seam so the Realm Authority never needs private key bytes.
func SignRevocationWith(rev identityapi.CapabilityRevocation, sign func([]byte) ([]byte, error)) (identityapi.CapabilityRevocation, error) {
	if sign == nil {
		return identityapi.CapabilityRevocation{}, fmt.Errorf("issuer signer is required")
	}
	raw, err := canonicalRevocation(rev)
	if err != nil {
		return identityapi.CapabilityRevocation{}, err
	}
	rev.Signature, err = sign(digest(revocationSignatureDomain, raw))
	if err != nil || len(rev.Signature) != ed25519.SignatureSize {
		return identityapi.CapabilityRevocation{}, fmt.Errorf("issuer signing failed")
	}
	return rev, nil
}

func verifyGrantSignature(grant identityapi.CapabilityGrant, public ed25519.PublicKey) error {
	raw, err := canonicalGrant(grant)
	if err != nil {
		return err
	}
	if len(public) != ed25519.PublicKeySize ||
		!ed25519.Verify(public, digest(grantSignatureDomain, raw), grant.Signature) {
		return fmt.Errorf("capability grant signature is invalid")
	}
	return nil
}

func verifyRevocationSignature(rev identityapi.CapabilityRevocation, public ed25519.PublicKey) error {
	raw, err := canonicalRevocation(rev)
	if err != nil {
		return err
	}
	if len(public) != ed25519.PublicKeySize ||
		!ed25519.Verify(public, digest(revocationSignatureDomain, raw), rev.Signature) {
		return fmt.Errorf("capability revocation signature is invalid")
	}
	return nil
}

func canonicalGrant(grant identityapi.CapabilityGrant) ([]byte, error) {
	var out bytes.Buffer
	writeUint32(&out, grant.Version)
	out.Write(grant.ChannelID[:])
	writeUint32(&out, grant.Generation)
	out.Write(grant.Secret.Bytes())
	out.Write(grant.GrantID[:])
	if err := writeString(&out, grant.IssuerPrincipal); err != nil {
		return nil, err
	}
	if err := writeString(&out, grant.SubjectPrincipal); err != nil {
		return nil, err
	}
	writeUint32(&out, uint32(grant.Permissions))
	if err := writeString(&out, string(grant.Scope)); err != nil {
		return nil, err
	}
	writeInt64(&out, grant.NotBefore.UTC().Unix())
	writeInt64(&out, grant.NotAfter.UTC().Unix())
	return out.Bytes(), nil
}

func canonicalRevocation(rev identityapi.CapabilityRevocation) ([]byte, error) {
	var out bytes.Buffer
	writeUint32(&out, rev.Version)
	out.Write(rev.GrantID[:])
	if err := writeString(&out, rev.IssuerPrincipal); err != nil {
		return nil, err
	}
	writeInt64(&out, rev.RevokedAt.UTC().Unix())
	return out.Bytes(), nil
}

func digest(domain string, raw []byte) []byte {
	sum := sha256.Sum256(append(append([]byte(domain), 0), raw...))
	return sum[:]
}

func writeString(out *bytes.Buffer, value string) error {
	if len(value) > int(^uint16(0)) {
		return fmt.Errorf("capability field is too long")
	}
	out.Write(binary.BigEndian.AppendUint16(nil, uint16(len(value))))
	out.WriteString(value)
	return nil
}

func writeUint32(out *bytes.Buffer, value uint32) {
	out.Write(binary.BigEndian.AppendUint32(nil, value))
}
func writeInt64(out *bytes.Buffer, value int64) {
	out.Write(binary.BigEndian.AppendUint64(nil, uint64(value)))
}
