package identity

// Summary is the public, non-secret identity of this Node. A Node has no
// Device unless an independent Credential is explicitly enrolled for it.
type Summary struct {
	Principal string
	PublicKey string
}

type Snapshot struct {
	State     string
	Principal string
	PublicKey string
	Source    string
}

func ProjectSnapshot(service Service) Snapshot {
	summary := service.NodeSummary()
	return Snapshot{State: service.NodeState(), Principal: summary.Principal, PublicKey: summary.PublicKey, Source: service.NodeSource()}
}

type StateStore interface {
	LoadIdentity() (principal string, publicKey string)
	SaveIdentity(principal string, publicKey string) error
}

type KeyStore interface {
	Load() (string, error)
	Save(privateKey string) error
}
