package authority

import (
	"encoding/json"
	"time"
)

type Status struct {
	Version               uint32    `json:"version"`
	SchemaVersion         uint32    `json:"schema_version"`
	RealmID               string    `json:"realm_id,omitempty"`
	RealmClass            string    `json:"realm_class,omitempty"`
	AuthorityEpoch        uint64    `json:"authority_epoch"`
	AuthoritySequence     uint64    `json:"authority_sequence"`
	CheckpointDigest      string    `json:"checkpoint_digest,omitempty"`
	Phase                 string    `json:"phase"`
	Readiness             string    `json:"readiness"`
	Reason                string    `json:"reason,omitempty"`
	MemberCount           uint32    `json:"member_count"`
	ChannelCount          uint32    `json:"channel_count"`
	PendingOperationCount uint32    `json:"pending_operation_count"`
	AuditOutboxDepth      uint32    `json:"audit_outbox_depth"`
	CurrentGeneration     uint32    `json:"current_generation"`
	OperationDeadline     time.Time `json:"operation_deadline,omitempty"`
}

func (s Status) String() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
