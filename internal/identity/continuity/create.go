package continuity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	identityprincipal "ardents/internal/identity/principal"
)

func (s *Service) createIdentity(store Store, keys KeyStore) (Summary, ed25519.PrivateKey, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Summary{}, nil, fmt.Errorf("generate key: %w", err)
	}

	sum := Summary{
		Principal: identityprincipal.DeriveID("p", public),
		Device:    identityprincipal.DeriveID("d", private.Seed()),
		PublicKey: base64.StdEncoding.EncodeToString(public),
	}

	if err := keys.Save(base64.StdEncoding.EncodeToString(private)); err != nil {
		return Summary{}, nil, err
	}
	if err := store.SaveIdentity(sum.Principal, sum.Device, sum.PublicKey); err != nil {
		return Summary{}, nil, err
	}

	s.state = "ready"
	s.source = "created"
	s.summary = sum
	return sum, private, nil
}
