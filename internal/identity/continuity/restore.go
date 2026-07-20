package continuity

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func (s *Service) loadReadyIdentity(store Store, keys KeyStore) (bool, string, error) {
	principal, device, publicKey := store.LoadIdentity()
	privateKey, err := keys.Load()
	if err != nil {
		return false, "", err
	}
	identityFields := 0
	for _, value := range []string{principal, device, publicKey} {
		if value != "" {
			identityFields++
		}
	}
	if identityFields == 0 && privateKey == "" {
		return false, "", nil
	}
	if identityFields == 3 && privateKey != "" {
		return true, privateKey, nil
	}
	return false, "", fmt.Errorf("identity continuity state is incomplete; restore matching state and key backup")
}

func (s *Service) restoreLoadedIdentity(store Store, privateKey string) (Summary, ed25519.PrivateKey, error) {
	principal, device, publicKey := store.LoadIdentity()
	raw, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return Summary{}, nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return Summary{}, nil, fmt.Errorf("identity private key has invalid length")
	}
	private := ed25519.PrivateKey(raw)
	public, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return Summary{}, nil, fmt.Errorf("identity public key is invalid")
	}
	if !private.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(public)) {
		return Summary{}, nil, fmt.Errorf("identity private key does not match persisted public key")
	}
	if err := validateDerivedIdentity(principal, device, private); err != nil {
		return Summary{}, nil, err
	}
	s.state = "ready"
	s.source = "restored"
	s.summary = Summary{
		Principal: principal,
		Device:    device,
		PublicKey: publicKey,
	}
	return s.summary, private, nil
}
