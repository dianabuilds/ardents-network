package migration

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	identityprincipal "ardents/internal/identity/principal"
)

const legacyPrincipalPrefix = "p_"

var errInvalidLegacyPrincipal = errors.New("legacy Principal identifier is invalid")

type LegacyPrincipalID struct{ digest [8]byte }

func ParseLegacyPrincipalID(value string) (LegacyPrincipalID, error) {
	if len(value) != 18 || !strings.HasPrefix(value, legacyPrincipalPrefix) || value != strings.ToLower(value) {
		return LegacyPrincipalID{}, errInvalidLegacyPrincipal
	}
	raw, err := hex.DecodeString(value[len(legacyPrincipalPrefix):])
	if err != nil || len(raw) != 8 {
		return LegacyPrincipalID{}, errInvalidLegacyPrincipal
	}
	var digest [8]byte
	copy(digest[:], raw)
	return LegacyPrincipalID{digest: digest}, nil
}

func LegacyPrincipalIDFromEd25519PublicKey(public ed25519.PublicKey) (LegacyPrincipalID, error) {
	if len(public) != ed25519.PublicKeySize {
		return LegacyPrincipalID{}, errInvalidLegacyPrincipal
	}
	sum := sha256.Sum256(public)
	var digest [8]byte
	copy(digest[:], sum[:8])
	return LegacyPrincipalID{digest: digest}, nil
}

func (id LegacyPrincipalID) String() string {
	return legacyPrincipalPrefix + hex.EncodeToString(id.digest[:])
}

func (id LegacyPrincipalID) MapToV1(public ed25519.PublicKey) (identityprincipal.ID, error) {
	derived, err := LegacyPrincipalIDFromEd25519PublicKey(public)
	if err != nil || derived != id {
		return identityprincipal.ID{}, errInvalidLegacyPrincipal
	}
	return identityprincipal.FromEd25519PublicKey(public)
}
