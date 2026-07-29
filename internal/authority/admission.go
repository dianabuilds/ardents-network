package authority

import (
	"context"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

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
	AdmitInitialGeneration(context.Context, Command) error
	AdmitChannelRotation(context.Context, Command) error
	AdmitChannelMembership(context.Context, Command) error
	AdmitChannelClass(context.Context, Command, identityapi.CapabilityScope) error
	AdmitAuthorityRecovery(context.Context, Command) error
}

// DeploymentFenceVerifier authenticates the protected control receipts that
// DeploymentFenceEvidence summarizes. Authority validates evidence shape and
// binding but must not treat caller-supplied digests as enforcement proof.
type DeploymentFenceVerifier interface {
	VerifyDeploymentFenceEvidence(
		context.Context,
		Command,
		DeploymentFenceEvidence,
	) error
}

type InitialGenerationRequest struct {
	Version              uint32
	RequestID            string
	RealmID              string
	ChannelClass         identityapi.CapabilityScope
	Permissions          identityapi.CapabilityPermission
	RecipientAttestation identityapi.CapabilityDeliveryAttestation
	ValidFor             time.Duration
}

type InitialGenerationResult struct {
	Version           uint32
	RealmID           string
	OperationID       string
	DeliveryID        string
	AuthoritySequence uint64
	ChannelID         [16]byte
	Generation        uint32
	Sealed            identitycapability.SealedGenerationDelivery
}

type InitialGenerationAcknowledgeRequest struct {
	Version uint32
	RealmID string
	Receipt identitycapability.GenerationDeliveryReceipt
}

type InitialGenerationAcknowledgeResult struct {
	Version           uint32
	RealmID           string
	DeliveryID        string
	AuthoritySequence uint64
	Phase             string
}
