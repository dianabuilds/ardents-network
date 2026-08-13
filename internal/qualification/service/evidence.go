package service

type candidate struct {
	Schema                string               `json:"schema"`
	SourceCommit          string               `json:"source_commit"`
	ImageID               string               `json:"image_id"`
	ManifestDigest        string               `json:"manifest_digest"`
	EvidenceDigest        string               `json:"evidence_digest"`
	NetworkID             [32]byte             `json:"network_id"`
	AuthorityPublic       [32]byte             `json:"authority_public"`
	IntroductionPublic    [32]byte             `json:"introduction_public"`
	RouteManifestDigest   [32]byte             `json:"route_manifest_digest"`
	Target                [32]byte             `json:"target"`
	Topology              string               `json:"topology"`
	Generations           []generationEvidence `json:"generations"`
	Negatives             map[string]bool      `json:"negatives"`
	NegativeMechanisms    map[string]string    `json:"negative_mechanisms"`
	OperationObservations map[string]bool      `json:"operation_observations"`
	OperationClasses      map[string]string    `json:"operation_classes"`
	OperationCounts       map[string]uint32    `json:"operation_counts"`
	ShortcutsAbsent       map[string]bool      `json:"shortcuts_absent"`
	Cleanup               map[string]bool      `json:"cleanup"`
	PrivateMaterialAbsent bool                 `json:"private_material_absent"`
	CleanupObservation    cleanupObservation   `json:"cleanup_observation"`
}

type cleanupObservation struct {
	Observed      bool     `json:"observed"`
	Project       string   `json:"project"`
	FixtureAbsent bool     `json:"fixture_absent"`
	Containers    []string `json:"containers"`
	Networks      []string `json:"networks"`
	Volumes       []string `json:"volumes"`
}

type generationEvidence struct {
	Generation                  uint64              `json:"generation"`
	Credential                  publicCredential    `json:"credential"`
	IntroductionAcknowledgement []byte              `json:"introduction_acknowledgement"`
	PublicationReady            bool                `json:"publication_ready"`
	ClientEndpoint              endpointEvidence    `json:"client_endpoint"`
	PublisherEndpoint           endpointEvidence    `json:"publisher_endpoint"`
	ClientApplication           applicationEvidence `json:"client_application"`
	PublisherApplication        applicationEvidence `json:"publisher_application"`
	Roles                       []roleEvidence      `json:"roles"`
	ContainerIDs                []string            `json:"container_ids"`
	ClientGrant                 grantEvidence       `json:"client_grant"`
	PublisherGrant              grantEvidence       `json:"publisher_grant"`
}

type grantEvidence struct {
	Broker    [32]byte `json:"broker"`
	Principal [32]byte `json:"principal"`
	Surface   string   `json:"surface"`
}

type publicCredential struct {
	AuthorityPublic [32]byte `json:"authority_public"`
	Target          [32]byte `json:"target"`
	InstancePublic  [32]byte `json:"instance_public"`
	Generation      uint64   `json:"generation"`
	NotBefore       int64    `json:"not_before"`
	NotAfter        int64    `json:"not_after"`
	NetworkID       [32]byte `json:"network_id"`
	Capabilities    uint32   `json:"capabilities"`
	Signature       [64]byte `json:"signature"`
}

type endpointEvidence struct {
	Class               string   `json:"class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	Generation          uint64   `json:"generation"`
	AcceptedBytes       uint32   `json:"accepted_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	ConnectionCanary    [32]byte `json:"connection_canary"`
	PrincipalCommitment [32]byte `json:"principal_commitment"`
	SessionCommitment   [32]byte `json:"session_commitment"`
	GrantSurface        string   `json:"grant_surface"`
	SessionConsumed     bool     `json:"session_consumed"`
	BrokerCommitment    [32]byte `json:"broker_commitment"`
	GrantCommitment     [32]byte `json:"grant_commitment"`
	SessionIssuedAt     int64    `json:"session_issued_at"`
	SessionExpiresAt    int64    `json:"session_expires_at"`
	MemoryHighWater     uint64   `json:"memory_high_water"`
	CPUSeconds          float64  `json:"cpu_seconds"`
	OpenFilesHighWater  uint32   `json:"open_files_high_water"`
	GoroutinesHighWater uint32   `json:"goroutines_high_water"`
	ActiveSessions      uint32   `json:"active_sessions"`
	TimerHighWater      uint32   `json:"timer_high_water"`
	QueueHighWater      uint32   `json:"queue_high_water"`
	TempEntries         uint32   `json:"temp_entries"`
}

type applicationEvidence struct {
	Schema              string   `json:"schema"`
	Role                string   `json:"role"`
	Terminal            string   `json:"terminal"`
	SentBytes           uint32   `json:"sent_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	SentDigest          [32]byte `json:"sent_digest"`
	ReceivedDigest      [32]byte `json:"received_digest"`
	ResultClass         string   `json:"result_class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	SendSeed            [32]byte `json:"send_seed"`
	ExpectSeed          [32]byte `json:"expect_seed"`
}

type roleEvidence struct {
	Role                string   `json:"role"`
	RuntimeID           string   `json:"runtime_id"`
	Terminal            string   `json:"terminal"`
	PID                 int      `json:"pid"`
	Cleanup             bool     `json:"cleanup"`
	ManifestDigest      [32]byte `json:"manifest_digest"`
	NetworkID           [32]byte `json:"network_id"`
	OpaqueBytes         uint64   `json:"opaque_bytes"`
	SourceID            string   `json:"source_id"`
	BuildDigest         [32]byte `json:"build_digest"`
	OpaqueDigest        [32]byte `json:"opaque_digest"`
	ReverseOpaqueBytes  uint64   `json:"reverse_opaque_bytes"`
	ReverseOpaqueDigest [32]byte `json:"reverse_opaque_digest"`
}

var routeRoles = [...]string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"}

var requiredNegatives = [...]string{
	"ungranted-sibling", "session-replay", "principal-substitution", "restart-reuse",
	"connection-admin", "credential-only", "wrong-target", "wrong-key", "expired",
	"wrong-network", "stale-generation", "same-generation-conflict",
	"not-yet-valid", "wrong-capability", "malformed-publication", "administration-connection",
	"administration-custody", "administration-export", "pid-substitution", "container-substitution",
	"malformed-ipc-frame", "oversized-ipc-frame", "partial-ipc-frame", "slow-ipc-frame",
	"stale-generation-new-work",
}

var expectedNegativeMechanisms = map[string]string{
	"ungranted-sibling": "hostile-sibling-volume-boundary", "session-replay": "serviceconn-session-replay",
	"principal-substitution": "serviceconn-principal-swap", "restart-reuse": "serviceconn-endpoint-restart",
	"connection-admin": "serviceconn-connection-publish", "credential-only": "serviceconn-no-admin-session",
	"wrong-target": "publication-target-mutation", "wrong-key": "instance-proof-key-mismatch",
	"expired": "credential-expired-time", "wrong-network": "credential-network-mutation",
	"stale-generation": "publication-old-generation", "same-generation-conflict": "publication-generation-conflict",
	"not-yet-valid": "credential-future-time", "wrong-capability": "credential-capability-mask",
	"malformed-publication": "publication-binary-truncation", "administration-connection": "serviceconn-admin-connect",
	"administration-custody": "serviceconn-admin-custody", "administration-export": "serviceconn-admin-export",
	"pid-substitution": "process-pid-derived-principal", "container-substitution": "broker-container-identity-swap",
	"malformed-ipc-frame": "unix-control-malformed-frame", "oversized-ipc-frame": "unix-control-oversized-frame",
	"partial-ipc-frame": "unix-control-partial-eof", "slow-ipc-frame": "unix-control-stalled-deadline",
	"stale-generation-new-work": "retired-runtime-new-connection",
}

var requiredShortcuts = [...]string{
	"direct", "shortened", "localhost-data", "shared-data-file", "dns", "proxy", "ambient-network", "route-visible-to-application",
}

var requiredCleanup = [...]string{
	"containers", "network", "listeners", "sockets", "processes", "sessions", "publications",
}
