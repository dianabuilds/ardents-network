package authority

import (
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

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
	Version   uint32 `json:"version"`
	Principal string `json:"principal,omitempty"`
}

type ChannelRecord struct {
	Version                        uint32                `json:"version"`
	ID                             [16]byte              `json:"id,omitempty"`
	Class                          string                `json:"class"`
	MemberCount                    uint32                `json:"member_count"`
	CurrentGeneration              uint32                `json:"current_generation"`
	PendingGenerationCount         uint32                `json:"pending_generation_count"`
	PreviousReceiveGenerationCount uint32                `json:"previous_receive_generation_count"`
	OutstandingDeliveryCount       uint32                `json:"outstanding_delivery_count"`
	Grant                          CapabilityGrantRecord `json:"grant,omitempty"`
}

type CapabilityGrantRecord struct {
	Version          uint32                           `json:"version"`
	ChannelID        [16]byte                         `json:"channel_id"`
	Generation       uint32                           `json:"generation"`
	Secret           []byte                           `json:"secret"`
	GrantID          [16]byte                         `json:"grant_id"`
	IssuerPrincipal  string                           `json:"issuer_principal"`
	SubjectPrincipal string                           `json:"subject_principal"`
	Permissions      identityapi.CapabilityPermission `json:"permissions"`
	Scope            identityapi.CapabilityScope      `json:"scope"`
	NotBefore        time.Time                        `json:"not_before"`
	NotAfter         time.Time                        `json:"not_after"`
	Signature        []byte                           `json:"signature"`
}

type InitialGenerationDeliveryRecord struct {
	Version            uint32                                       `json:"version"`
	RequestID          string                                       `json:"request_id"`
	PayloadHash        string                                       `json:"payload_hash"`
	OperationID        string                                       `json:"operation_id"`
	DeliveryID         string                                       `json:"delivery_id"`
	ChannelID          [16]byte                                     `json:"channel_id"`
	RecipientPrincipal string                                       `json:"recipient_principal"`
	Phase              string                                       `json:"phase"`
	RetryGeneration    uint32                                       `json:"retry_generation"`
	ReceiptKey         []byte                                       `json:"receipt_key"`
	Sealed             identitycapability.SealedGenerationDelivery  `json:"sealed"`
	Receipt            identitycapability.GenerationDeliveryReceipt `json:"receipt,omitempty"`
	CreatedAt          time.Time                                    `json:"created_at"`
	Deadline           time.Time                                    `json:"deadline"`
}

type Ledger struct {
	Version                     uint32                            `json:"version"`
	SchemaVersion               uint32                            `json:"schema_version"`
	Revision                    uint64                            `json:"revision"`
	RealmID                     string                            `json:"realm_id"`
	RealmClass                  string                            `json:"realm_class"`
	AuthorityPrincipal          string                            `json:"authority_principal"`
	AuthorityPublicKey          []byte                            `json:"authority_public_key"`
	AuthorityEpoch              uint64                            `json:"authority_epoch"`
	AuthoritySequence           uint64                            `json:"authority_sequence"`
	Phase                       string                            `json:"phase"`
	Readiness                   string                            `json:"readiness"`
	Reason                      string                            `json:"reason,omitempty"`
	AuditHead                   string                            `json:"audit_head"`
	GenesisCheckpointDigest     string                            `json:"genesis_checkpoint_digest,omitempty"`
	Checkpoint                  SignedCheckpoint                  `json:"checkpoint"`
	Members                     []MemberRecord                    `json:"members"`
	Channels                    []ChannelRecord                   `json:"channels"`
	Operations                  []OperationRecord                 `json:"operations"`
	Idempotency                 []IdempotencyRecord               `json:"idempotency"`
	AuditLog                    []AuditRecord                     `json:"audit_log"`
	AuditOutbox                 []AuditRecord                     `json:"audit_outbox"`
	InitialGenerationDeliveries []InitialGenerationDeliveryRecord `json:"initial_generation_deliveries,omitempty"`
}

func capabilityGrantRecord(grant identityapi.CapabilityGrant) CapabilityGrantRecord {
	return CapabilityGrantRecord{
		Version: grant.Version, ChannelID: grant.ChannelID,
		Generation: grant.Generation, Secret: grant.Secret.Bytes(), GrantID: grant.GrantID,
		IssuerPrincipal: grant.IssuerPrincipal, SubjectPrincipal: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope,
		NotBefore: grant.NotBefore, NotAfter: grant.NotAfter,
		Signature: append([]byte(nil), grant.Signature...),
	}
}

func (record CapabilityGrantRecord) restore() (identityapi.CapabilityGrant, bool) {
	secret, ok := identityapi.NewCapabilitySecret(record.Secret)
	if !ok {
		return identityapi.CapabilityGrant{}, false
	}
	return identityapi.CapabilityGrant{
		Version: record.Version, ChannelID: record.ChannelID,
		Generation: record.Generation, Secret: secret, GrantID: record.GrantID,
		IssuerPrincipal: record.IssuerPrincipal, SubjectPrincipal: record.SubjectPrincipal,
		Permissions: record.Permissions, Scope: record.Scope,
		NotBefore: record.NotBefore, NotAfter: record.NotAfter,
		Signature: append([]byte(nil), record.Signature...),
	}, true
}
