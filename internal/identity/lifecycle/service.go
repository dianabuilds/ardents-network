package lifecycle

import (
	"crypto/ed25519"

	identityapi "ardents/internal/identity/api"
	identitycontinuity "ardents/internal/identity/continuity"
)

type Service struct {
	continuity *identitycontinuity.Service
}

func New() *Service {
	return &Service{continuity: identitycontinuity.New()}
}

func (s *Service) Ensure(store Store, keys KeyStore) (Summary, ed25519.PrivateKey, error) {
	return s.continuity.Ensure(store, keys)
}

func (s *Service) EnsureNode(store identityapi.Store, keys identityapi.KeyStore) (identityapi.Summary, ed25519.PrivateKey, error) {
	return s.Ensure(store, keys)
}

func (s *Service) State() string {
	return s.continuity.State()
}

func (s *Service) Summary() Summary {
	return s.continuity.Summary()
}

func (s *Service) NodeSummary() identityapi.Summary {
	return s.Summary()
}

func (s *Service) Source() string {
	return s.continuity.Source()
}

func (s *Service) NodeSource() string {
	return s.Source()
}

func (s *Service) NodeState() string {
	return s.State()
}

func (s *Service) NormalizeSubject(call identityapi.CallContext) identityapi.Subject {
	return identityapi.NormalizeSubject(call)
}

func (s *Service) Authorize(call identityapi.CallContext, domain string, access identityapi.Access) identityapi.Decision {
	return identityapi.Authorize(call, domain, access)
}

func (s *Service) AuthorizeSubject(subject identityapi.Subject, domain string, access identityapi.Access) identityapi.Decision {
	return identityapi.AuthorizeSubject(subject, domain, access)
}
