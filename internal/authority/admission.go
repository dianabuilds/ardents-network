package authority

import "context"

type Command struct {
	Actor, Effective                 string
	Action, ResourceKind, ResourceID string
}

type CreateRequest struct {
	Version    uint32
	RequestID  string
	RealmClass string
}

type CreateResult struct {
	Version           uint32 `json:"version"`
	RealmID           string `json:"realm_id"`
	OperationID       string `json:"operation_id"`
	AuthorityEpoch    uint64 `json:"authority_epoch"`
	AuthoritySequence uint64 `json:"authority_sequence"`
	CheckpointDigest  string `json:"checkpoint_digest"`
	Phase             string `json:"phase"`
}

type InspectRequest struct {
	Version uint32
	RealmID string
}

type ProductPolicy interface {
	AdmitRealmGenesis(context.Context, Command) error
}
