package authority

import "time"

type OperationRecord struct {
	Version   uint32    `json:"version"`
	ID        string    `json:"id"`
	RequestID string    `json:"request_id"`
	Kind      string    `json:"kind"`
	Phase     string    `json:"phase"`
	CreatedAt time.Time `json:"created_at"`
	Deadline  time.Time `json:"deadline"`
}

type AuditRecord struct {
	Version      uint32    `json:"version"`
	ID           string    `json:"id"`
	Actor        string    `json:"actor"`
	Effective    string    `json:"effective"`
	Action       string    `json:"action"`
	ResourceKind string    `json:"resource_kind"`
	ResourceID   string    `json:"resource_id"`
	OperationID  string    `json:"operation_id"`
	Outcome      string    `json:"outcome"`
	PreviousHash string    `json:"previous_hash,omitempty"`
	Hash         string    `json:"hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type IdempotencyRecord struct {
	Version     uint32       `json:"version"`
	RequestID   string       `json:"request_id"`
	PayloadHash string       `json:"payload_hash"`
	Result      CreateResult `json:"result"`
}

type MemberRecord struct {
	Version uint32 `json:"version"`
}

type ChannelRecord struct {
	Version                        uint32 `json:"version"`
	Class                          string `json:"class"`
	MemberCount                    uint32 `json:"member_count"`
	CurrentGeneration              uint32 `json:"current_generation"`
	PendingGenerationCount         uint32 `json:"pending_generation_count"`
	PreviousReceiveGenerationCount uint32 `json:"previous_receive_generation_count"`
	OutstandingDeliveryCount       uint32 `json:"outstanding_delivery_count"`
}

type Ledger struct {
	Version            uint32              `json:"version"`
	SchemaVersion      uint32              `json:"schema_version"`
	Revision           uint64              `json:"revision"`
	RealmID            string              `json:"realm_id"`
	RealmClass         string              `json:"realm_class"`
	AuthorityPrincipal string              `json:"authority_principal"`
	AuthorityPublicKey []byte              `json:"authority_public_key"`
	AuthorityEpoch     uint64              `json:"authority_epoch"`
	AuthoritySequence  uint64              `json:"authority_sequence"`
	Phase              string              `json:"phase"`
	Readiness          string              `json:"readiness"`
	Reason             string              `json:"reason,omitempty"`
	AuditHead          string              `json:"audit_head"`
	Checkpoint         SignedCheckpoint    `json:"checkpoint"`
	Members            []MemberRecord      `json:"members"`
	Channels           []ChannelRecord     `json:"channels"`
	Operations         []OperationRecord   `json:"operations"`
	Idempotency        []IdempotencyRecord `json:"idempotency"`
	AuditLog           []AuditRecord       `json:"audit_log"`
	AuditOutbox        []AuditRecord       `json:"audit_outbox"`
}
