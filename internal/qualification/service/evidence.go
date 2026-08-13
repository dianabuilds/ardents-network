package service

type candidate struct {
	Schema                string               `json:"schema"`
	SourceCommit          string               `json:"source_commit"`
	ImageID               string               `json:"image_id"`
	ManifestDigest        string               `json:"manifest_digest"`
	EvidenceDigest        string               `json:"evidence_digest"`
	NetworkID             [32]byte             `json:"network_id"`
	AuthorityPublic       [32]byte             `json:"authority_public"`
	Target                [32]byte             `json:"target"`
	Generations           []generationEvidence `json:"generations"`
	Negatives             map[string]bool      `json:"negatives"`
	ShortcutsAbsent       map[string]bool      `json:"shortcuts_absent"`
	Cleanup               map[string]bool      `json:"cleanup"`
	PrivateMaterialAbsent bool                 `json:"private_material_absent"`
}

type generationEvidence struct {
	Generation                  uint64              `json:"generation"`
	Credential                  publicCredential    `json:"credential"`
	IntroductionAcknowledgement [32]byte            `json:"introduction_acknowledgement"`
	PublicationReady            bool                `json:"publication_ready"`
	ClientEndpoint              endpointEvidence    `json:"client_endpoint"`
	PublisherEndpoint           endpointEvidence    `json:"publisher_endpoint"`
	ClientApplication           applicationEvidence `json:"client_application"`
	PublisherApplication        applicationEvidence `json:"publisher_application"`
	Roles                       []roleEvidence      `json:"roles"`
	ContainerIDs                []string            `json:"container_ids"`
}

type publicCredential struct {
	AuthorityPublic [32]byte `json:"authority_public"`
	Target          [32]byte `json:"target"`
	InstancePublic  [32]byte `json:"instance_public"`
	NetworkID       [32]byte `json:"network_id"`
	Generation      uint64   `json:"generation"`
	NotBefore       int64    `json:"not_before"`
	NotAfter        int64    `json:"not_after"`
	Capabilities    uint32   `json:"capabilities"`
	Signature       [64]byte `json:"signature"`
}

type endpointEvidence struct {
	Class               string   `json:"class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	Generation          uint64   `json:"generation"`
	AcceptedBytes       uint32   `json:"accepted_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
}

type applicationEvidence struct {
	Terminal       string   `json:"terminal"`
	SentBytes      uint32   `json:"sent_bytes"`
	ReceivedBytes  uint32   `json:"received_bytes"`
	SentDigest     [32]byte `json:"sent_digest"`
	ReceivedDigest [32]byte `json:"received_digest"`
}

type roleEvidence struct {
	Role      string `json:"role"`
	RuntimeID string `json:"runtime_id"`
	Terminal  string `json:"terminal"`
	PID       int    `json:"pid"`
	Cleanup   bool   `json:"cleanup"`
}

var routeRoles = [...]string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"}

var requiredNegatives = [...]string{
	"ungranted-sibling", "session-replay", "principal-substitution", "restart-reuse",
	"connection-admin", "credential-only", "wrong-target", "wrong-key", "expired",
	"wrong-network", "stale-generation", "same-generation-conflict",
}

var requiredShortcuts = [...]string{
	"direct", "shortened", "localhost-data", "shared-data-file", "dns", "proxy", "ambient-network", "route-visible-to-application",
}

var requiredCleanup = [...]string{
	"containers", "network", "listeners", "sockets", "processes", "sessions", "publications",
}
