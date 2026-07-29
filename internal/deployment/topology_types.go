package deployment

const (
	// TopologyVersion is the only manifest schema accepted by MR-01.
	TopologyVersion = "ardents.topology/v1"
	// PlanVersion identifies the deterministic redacted MR-01 output.
	PlanVersion = "ardents.topology.plan/v1"
)

type topologyManifest struct {
	APIVersion           string               `json:"api_version"`
	Mode                 string               `json:"mode"`
	TransportProfile     string               `json:"transport_profile"`
	OperatorSignerAlias  string               `json:"operator_signer_alias"`
	Clock                clockSpec            `json:"clock"`
	Authority            authoritySpec        `json:"authority"`
	CheckpointRepository checkpointRepository `json:"checkpoint_repository"`
	SignedDNSRoots       []string             `json:"signed_dns_roots"`
	Nodes                []nodeSpec           `json:"nodes"`
}

type clockSpec struct {
	MaxSkewSeconds               int `json:"max_skew_seconds"`
	AuthoritySafetyMarginSeconds int `json:"authority_safety_margin_seconds"`
}

type failureDomain struct {
	Class string `json:"class"`
	ID    string `json:"id"`
}

type authoritySpec struct {
	Slot                string        `json:"slot"`
	RealmID             string        `json:"realm_id"`
	StateRef            string        `json:"state_ref"`
	FailureDomain       failureDomain `json:"failure_domain"`
	BackupRef           string        `json:"backup_ref"`
	BackupFailureDomain failureDomain `json:"backup_failure_domain"`
}

type checkpointRepository struct {
	Reference        string        `json:"reference"`
	FailureDomain    failureDomain `json:"failure_domain"`
	ImmutableHistory bool          `json:"immutable_history"`
	MaxHeads         int           `json:"max_heads"`
}

type nodeSpec struct {
	Slot                  string      `json:"slot"`
	Profile               string      `json:"profile"`
	Host                  hostSpec    `json:"host"`
	NodeStateRef          string      `json:"node_state_ref"`
	ExpectedNodePrincipal string      `json:"expected_node_principal"`
	ExpectedWakuPeerID    string      `json:"expected_waku_peer_id"`
	Image                 string      `json:"image"`
	Bootstrap             bool        `json:"bootstrap"`
	Store                 storeSpec   `json:"store"`
	Ingress               ingressSpec `json:"ingress"`
	StaticRecoveryPeers   []string    `json:"static_recovery_peers"`
}

type hostSpec struct {
	OS            string        `json:"os"`
	Arch          string        `json:"arch"`
	Ownership     string        `json:"ownership"`
	SSHAlias      string        `json:"ssh_alias"`
	HostKeyPinRef string        `json:"host_key_pin_ref"`
	FailureDomain failureDomain `json:"failure_domain"`
}

type storeSpec struct {
	Persistent     bool   `json:"persistent"`
	RetentionClass string `json:"retention_class"`
}

type ingressSpec struct {
	Kind                string  `json:"kind"`
	Address             *string `json:"address"`
	CertificateRef      *string `json:"certificate_ref"`
	CertificateIdentity *string `json:"certificate_identity"`
}

// Plan is the ordinary redacted projection of one admitted topology.
type Plan struct {
	APIVersion         string        `json:"api_version"`
	Mode               string        `json:"mode"`
	TransportProfile   string        `json:"transport_profile"`
	SignedDNSRootCount int           `json:"signed_dns_root_count"`
	Authority          AuthorityPlan `json:"authority"`
	Hosts              []HostPlan    `json:"hosts"`
}

// AuthorityPlan records only non-sensitive placement invariants.
type AuthorityPlan struct {
	Slot                            string `json:"slot"`
	SeparateConsistencyGroup        bool   `json:"separate_consistency_group"`
	IndependentBackup               bool   `json:"independent_backup"`
	IndependentCheckpointRepository bool   `json:"independent_checkpoint_repository"`
	CheckpointMaxHeads              int    `json:"checkpoint_max_heads"`
	MaxClockSkewSeconds             int    `json:"max_clock_skew_seconds"`
	AuthoritySafetyMarginSeconds    int    `json:"authority_safety_margin_seconds"`
}

// HostPlan records bounded later checks without protected host identifiers.
type HostPlan struct {
	Slot                string   `json:"slot"`
	Role                string   `json:"role"`
	Profile             string   `json:"profile"`
	TransportProfile    string   `json:"transport_profile"`
	Ingress             string   `json:"ingress"`
	Bootstrap           bool     `json:"bootstrap"`
	PersistentStore     bool     `json:"persistent_store"`
	StoreRetentionClass string   `json:"store_retention_class"`
	StaticRecoveryPeers []string `json:"static_recovery_peers"`
	Checks              []string `json:"checks"`
}
