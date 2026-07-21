package identity

import "crypto/ed25519"

type NodeService struct {
	state   string
	summary Summary
	source  string
}

func NewService() *NodeService {
	return &NodeService{state: "new"}
}

func (s *NodeService) Ensure(store StateStore, keys KeyStore) (Summary, ed25519.PrivateKey, error) {
	ready, privateKey, err := s.loadReadyIdentity(store, keys)
	if err != nil {
		return Summary{}, nil, err
	}
	if ready {
		return s.restoreLoadedIdentity(store, privateKey)
	}
	return s.createIdentity(store, keys)
}

func (s *NodeService) EnsureNode(store StateStore, keys KeyStore) (Summary, ed25519.PrivateKey, error) {
	return s.Ensure(store, keys)
}

func (s *NodeService) State() string {
	return s.state
}

func (s *NodeService) Summary() Summary {
	return s.summary
}

func (s *NodeService) Source() string {
	return s.source
}

func (s *NodeService) NodeSummary() Summary { return s.Summary() }
func (s *NodeService) NodeSource() string   { return s.Source() }
func (s *NodeService) NodeState() string    { return s.State() }

func (s *NodeService) NormalizeSubject(call CallContext) Subject {
	return NormalizeSubject(call)
}

func (s *NodeService) Authorize(call CallContext, domain string, access Access) Decision {
	return Authorize(call, domain, access)
}

func (s *NodeService) AuthorizeSubject(subject Subject, domain string, access Access) Decision {
	return AuthorizeSubject(subject, domain, access)
}
