package stage6verify

type controlRoleEvidence struct {
	Network          [32]byte
	Exchanges        []controlExchangeEvidence
	RelayRequests    uint32
	GatewayRequests  uint32
	GatewayAccepted  uint32
	ClaimEpoch       uint64
	ClaimMaximum     uint32
	ClaimThreshold   int
	ClaimAuthorities []byte
}

type controlExchangeEvidence struct {
	Isolation [32]byte
	Admission admissionProof
	Envelope  []byte
	Operation controlOperationEvidence
	Result    controlExecutionResult
}

type controlExecutionResult struct {
	Class      string
	Generation uint64
	Revision   uint64
	State      []byte
}

type controlOperationEvidence struct {
	Kind               string   `json:"kind"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network,omitempty"`
	Nonce              [32]byte `json:"nonce,omitempty"`
	Deadline           int64    `json:"deadline,omitempty"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name,omitempty"`
	Generation         uint64   `json:"generation,omitempty"`
	ExpectedRevision   uint64   `json:"expected_revision,omitempty"`
	ParentGeneration   uint64   `json:"parent_generation,omitempty"`
	ParentRevision     uint64   `json:"parent_revision,omitempty"`
	ChildGeneration    uint64   `json:"child_generation,omitempty"`
	Authority          [32]byte `json:"authority,omitempty"`
	SuccessorAuthority [32]byte `json:"successor_authority,omitempty"`
	Target             [32]byte `json:"target,omitempty"`
	LeaseNotAfter      int64    `json:"lease_not_after,omitempty"`
	RecordNotAfter     int64    `json:"record_not_after,omitempty"`
	PolicyNotBefore    int64    `json:"policy_not_before,omitempty"`
	RecoveryNotBefore  int64    `json:"recovery_not_before,omitempty"`
	PolicyID           [32]byte `json:"policy_id,omitempty"`
	RecoveryStep       string   `json:"recovery_step,omitempty"`
	OrderingProof      []byte   `json:"ordering_proof,omitempty"`
	AuthorityProof     []byte   `json:"authority_proof,omitempty"`
	RecoveryPolicy     []byte   `json:"recovery_policy,omitempty"`
	RecoveryProof      []byte   `json:"recovery_proof,omitempty"`
}

func verifyControlRoleTrace(trace traceRecord, secret [32]byte) bool {
	var evidence controlRoleEvidence
	before, err := decodeRecords(trace.Input)
	if err != nil || len(before) != 11 {
		return false
	}
	after, err := decodeRecords(trace.Output)
	if err != nil || len(after) != 12 || decodeNestedJSON(trace.Auxiliary, &evidence) != nil ||
		evidence.Network == [32]byte{} || len(evidence.Exchanges) != 12 ||
		evidence.RelayRequests != 12 || evidence.GatewayRequests != 12 || evidence.GatewayAccepted != 12 ||
		evidence.ClaimEpoch == 0 || evidence.ClaimMaximum == 0 || evidence.ClaimThreshold < 2 ||
		len(evidence.ClaimAuthorities) == 0 || len(evidence.ClaimAuthorities)%64 != 0 ||
		!equalStrings(trace.Fields, []string{"relay-location-only", "authority-control-only", "complete-control-lifecycle", "admitted"}) {
		return false
	}
	return verifyControlExchanges(evidence, before, after, secret)
}
