package identity

import (
	"crypto/ed25519"
	"fmt"

	identityprincipal "ardents/internal/identity/principal"
)

func validateDerivedIdentity(principal, device string, private ed25519.PrivateKey) error {
	public := private.Public().(ed25519.PublicKey)
	derived, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil || principal != derived.String() {
		return fmt.Errorf("identity principal does not match persisted key")
	}
	if device != "" {
		return fmt.Errorf("node identity cannot claim a Device without a Credential")
	}
	return nil
}
