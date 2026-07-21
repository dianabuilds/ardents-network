package identity

import (
	"crypto/ed25519"
	"fmt"

	identityprincipal "ardents/internal/identity/principal"
)

func validateDerivedIdentity(principal, device string, private ed25519.PrivateKey) error {
	public := private.Public().(ed25519.PublicKey)
	if principal != identityprincipal.DeriveID("p", public) {
		return fmt.Errorf("identity principal does not match persisted key")
	}
	if device != identityprincipal.DeriveID("d", private.Seed()) {
		return fmt.Errorf("identity device does not match persisted key")
	}
	return nil
}
