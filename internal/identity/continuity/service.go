package continuity

import "crypto/ed25519"

type Service struct {
	state   string
	summary Summary
	source  string
}

func New() *Service {
	return &Service{state: "new"}
}

func (s *Service) Ensure(store Store, keys KeyStore) (Summary, ed25519.PrivateKey, error) {
	ready, privateKey, err := s.loadReadyIdentity(store, keys)
	if err != nil {
		return Summary{}, nil, err
	}
	if ready {
		return s.restoreLoadedIdentity(store, privateKey)
	}
	return s.createIdentity(store, keys)
}

func (s *Service) State() string {
	return s.state
}

func (s *Service) Summary() Summary {
	return s.summary
}

func (s *Service) Source() string {
	return s.source
}
