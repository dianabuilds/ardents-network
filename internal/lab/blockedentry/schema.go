package blockedentry

const developmentFixtureProfile = "h3-s5-b1-development-fixture-v1"

type manifest struct {
	Schema                string               `json:"schema"`
	CampaignKind          string               `json:"campaign_kind"`
	Profile               string               `json:"profile"`
	RunID                 string               `json:"run_id"`
	SourceIdentity        string               `json:"source_identity"`
	SupplyClass           string               `json:"supply_class"`
	FixtureMode           string               `json:"fixture_mode"`
	HarnessSHA256         string               `json:"harness_sha256"`
	RunnerSHA256          string               `json:"runner_sha256"`
	VerifierSHA256        string               `json:"verifier_sha256"`
	ClientSHA256          string               `json:"client_sha256"`
	ServerSHA256          string               `json:"server_sha256"`
	CanarySHA256          string               `json:"canary_sha256"`
	Groups                []manifestGroup      `json:"groups"`
	Topology              []topologyRole       `json:"topology"`
	Boundaries            []string             `json:"boundaries"`
	ResidualKinds         []string             `json:"residual_kinds"`
	SecretArtifacts       []artifactCommitment `json:"secret_artifacts"`
	SupplementalArtifacts []artifactCommitment `json:"supplemental_artifacts"`
	CreatedUnixNano       int64                `json:"created_unix_nano"`
	ManifestNonceHash     string               `json:"manifest_nonce_hash"`
	EvidenceRootHash      string               `json:"evidence_root_hash"`
	RegistryRootHash      string               `json:"registry_root_hash"`
	AttributionSources    []attributionSource  `json:"attribution_sources"`
}

type topologyRole struct {
	Role      string `json:"role"`
	Process   string `json:"process"`
	Namespace string `json:"namespace"`
	Network   string `json:"network"`
}

type manifestGroup struct {
	ID       string   `json:"id"`
	Variants []string `json:"variants"`
	Episodes int      `json:"episodes"`
}

type artifactCommitment struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type evidence struct {
	Schema                string               `json:"schema"`
	CampaignKind          string               `json:"campaign_kind"`
	Profile               string               `json:"profile"`
	RunID                 string               `json:"run_id"`
	ManifestSHA256        string               `json:"manifest_sha256"`
	Events                []event              `json:"events"`
	Observers             []observer           `json:"observers"`
	Cleanup               cleanupInventory     `json:"cleanup"`
	SecretArtifacts       []artifactCommitment `json:"secret_artifacts"`
	SupplementalArtifacts []artifactCommitment `json:"supplemental_artifacts"`
	AttributionArtifacts  []artifactCommitment `json:"attribution_artifacts"`
	CollectionClosed      bool                 `json:"collection_closed"`
}

type evidenceClosure struct {
	Schema         string `json:"schema"`
	RunID          string `json:"run_id"`
	ManifestSHA256 string `json:"manifest_sha256"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	ClosedUnixNano int64  `json:"closed_unix_nano"`
}

type event struct {
	ID                   string `json:"id"`
	Group                string `json:"group"`
	Variant              string `json:"variant"`
	Episode              int    `json:"episode"`
	ExpectedTerminal     string `json:"expected_terminal"`
	ObservedTerminal     string `json:"observed_terminal"`
	GatePassed           bool   `json:"gate_passed"`
	EvidenceTrustworthy  bool   `json:"evidence_trustworthy"`
	FaultOwner           string `json:"fault_owner"`
	AttributionEvidence  string `json:"attribution_evidence"`
	Diagnostic           string `json:"diagnostic"`
	CanarySetHash        string `json:"canary_set_hash"`
	StartedOffsetMillis  uint64 `json:"started_offset_millis"`
	TerminalOffsetMillis uint64 `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64 `json:"cleanup_offset_millis"`
	AdapterCleanupMillis uint64 `json:"adapter_cleanup_millis"`
}

type observer struct {
	Boundary             string `json:"boundary"`
	IPv4UDPControl       bool   `json:"ipv4_udp_control"`
	IPv6UDPControl       bool   `json:"ipv6_udp_control"`
	IPv4TCPControl       bool   `json:"ipv4_tcp_control"`
	Attribution          string `json:"attribution"`
	ForbiddenPackets     uint64 `json:"forbidden_packets"`
	ForbiddenOwner       string `json:"forbidden_owner"`
	UnclassifiedPackets  uint64 `json:"unclassified_packets"`
	ObservationCompleted bool   `json:"observation_completed"`
}

type cleanupInventory struct {
	Complete bool       `json:"complete"`
	Items    []residual `json:"items"`
}

type residual struct {
	Kind                string `json:"kind"`
	Count               uint64 `json:"count"`
	Owner               string `json:"owner"`
	AttributionEvidence string `json:"attribution_evidence"`
}

type canaryCorpus struct {
	Sets []canarySet `json:"sets"`
}

type canarySet struct {
	Variant     string `json:"variant"`
	Invite      string `json:"invite"`
	Address     string `json:"address"`
	Path        string `json:"path"`
	Certificate string `json:"certificate"`
}
