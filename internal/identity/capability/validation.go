package capability

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	identityapi "ardents/internal/identity"
	identityprincipal "ardents/internal/identity/principal"
)

func validateGrant(grant identityapi.CapabilityGrant, issuerPublic ed25519.PublicKey) error {
	if grant.Version != 1 || grant.Generation == 0 {
		return fmt.Errorf("capability grant version or generation is invalid")
	}
	if zeroID(grant.ChannelID) || zeroID(grant.GrantID) || !grant.Secret.Valid() {
		return fmt.Errorf("capability grant identifiers or secret are invalid")
	}
	if !knownScope(grant.Scope) || grant.Permissions == 0 ||
		grant.Permissions&^identityapi.CapabilityKnownPermissions != 0 {
		return fmt.Errorf("capability grant scope or permissions are invalid")
	}
	if !grant.NotBefore.Before(grant.NotAfter) {
		return fmt.Errorf("capability grant validity is invalid")
	}
	if grant.NotBefore.Nanosecond() != 0 || grant.NotAfter.Nanosecond() != 0 {
		return fmt.Errorf("capability grant validity must use whole Unix seconds")
	}
	if !validPrincipal(grant.SubjectPrincipal) || !validPrincipal(grant.IssuerPrincipal) {
		return fmt.Errorf("capability grant principal is invalid")
	}
	if identityprincipal.DeriveID("p", issuerPublic) != grant.IssuerPrincipal {
		return fmt.Errorf("capability grant issuer key does not match issuer")
	}
	return verifyGrantSignature(grant, issuerPublic)
}

func validateRevocation(rev identityapi.CapabilityRevocation, issuerPublic ed25519.PublicKey) error {
	if rev.Version != 1 || zeroID(rev.GrantID) || rev.RevokedAt.IsZero() ||
		rev.RevokedAt.Nanosecond() != 0 {
		return fmt.Errorf("capability revocation is invalid")
	}
	if identityprincipal.DeriveID("p", issuerPublic) != rev.IssuerPrincipal {
		return fmt.Errorf("capability revocation issuer key does not match issuer")
	}
	return verifyRevocationSignature(rev, issuerPublic)
}

func knownScope(scope identityapi.CapabilityScope) bool {
	switch scope {
	case identityapi.CapabilityRealmDiscovery,
		identityapi.CapabilityDataExchange,
		identityapi.CapabilityApplication,
		identityapi.CapabilityControl:
		return true
	default:
		return false
	}
}

func validPrincipal(value string) bool {
	if !strings.HasPrefix(value, "p_") || len(value) != 18 {
		return false
	}
	_, err := hex.DecodeString(value[2:])
	return err == nil
}

func zeroID(id [16]byte) bool {
	var combined byte
	for _, value := range id {
		combined |= value
	}
	return combined == 0
}
