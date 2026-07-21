package identity

import "crypto/ed25519"

type SubjectRef struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

type CallContext struct {
	Subject       SubjectRef     `json:"subject"`
	Capabilities  []string       `json:"capabilities,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Authenticated bool           `json:"authenticated,omitempty"`
}

func (c CallContext) CanonicalSubject() SubjectRef {
	return c.Subject
}

func (c CallContext) CanonicalCapabilities() []string {
	if len(c.Capabilities) == 0 {
		return nil
	}
	return append([]string(nil), c.Capabilities...)
}

type Summary struct {
	Principal string
	Device    string
	PublicKey string
}

type Snapshot struct {
	State     string
	Principal string
	Device    string
	PublicKey string
	Source    string
}

func ProjectSnapshot(service Service) Snapshot {
	summary := service.NodeSummary()
	return Snapshot{State: service.NodeState(), Principal: summary.Principal, Device: summary.Device, PublicKey: summary.PublicKey, Source: service.NodeSource()}
}

type StateStore interface {
	LoadIdentity() (principal string, device string, publicKey string)
	SaveIdentity(principal string, device string, publicKey string) error
}

type KeyStore interface {
	Load() (string, error)
	Save(privateKey string) error
}

type Subject struct {
	Ref           SubjectRef
	Authenticated bool
	Capabilities  []string
}

type Access string

const (
	AccessRead  Access = "read"
	AccessWrite Access = "write"
)

type Decision struct {
	Allowed bool
	Code    string
	Message string
	Reason  string
}

type Service interface {
	EnsureNode(StateStore, KeyStore) (Summary, ed25519.PrivateKey, error)
	NodeSummary() Summary
	NodeSource() string
	NodeState() string
	NormalizeSubject(CallContext) Subject
	Authorize(CallContext, string, Access) Decision
	AuthorizeSubject(Subject, string, Access) Decision
}
