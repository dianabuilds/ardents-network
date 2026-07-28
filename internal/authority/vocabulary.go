// Package authority owns the single-Realm Channel Grant Authority ledger,
// monotonic checkpoint contract, and bounded redacted status projection.
// It does not own Operator authentication, Product Policy, signer custody,
// checkpoint-repository administration, or deployment isolation execution.
package authority

import (
	"errors"
	"regexp"
	"time"

	identityapi "ardents/internal/identity"
)

type MembershipChangeKind string

const (
	ContractVersion uint32 = 1
	SchemaVersion   uint32 = 1

	MaxRealmMembers               = 256
	MaxActiveChannels             = 1024
	MaxMembersPerChannel          = 256
	MaxOutstandingDeliveries      = 4
	MaxPendingGenerations         = 1
	MaxPreviousReceiveGenerations = 1
	MaxOperations                 = 4096
	MaxAuditRecords               = 4096
	MaxAuditOutboxRecords         = 4096
	MaxIdempotencyRecords         = 4096
	MaxCheckpointRecords          = 4096
	MaxRequestIDBytes             = 128

	RealmClassProduction  = "production"
	MaxOperationLifetime  = 24 * time.Hour
	GrantValidity         = 30 * 24 * time.Hour
	GrantRenewalThreshold = 24 * time.Hour

	ActionCreate                = "realm.authority.create"
	ActionInspect               = "realm.channel.audit.read"
	ActionIssueDelivery         = "realm.channel.delivery.issue"
	ActionAcknowledgeDelivery   = "realm.channel.delivery.acknowledge"
	ActionRotateGeneration      = "realm.channel.generation.rotate"
	ActionChangeMembership      = "realm.channel.membership.change"
	ActionCommitActivation      = "realm.channel.activation.commit"
	ActionAcknowledgeActivation = "realm.channel.activation.acknowledge"
	ActionVerifyRestore         = "realm.authority.restore.verify"
	ActionPlanTransition        = "realm.authority.transition.plan"

	ResourceKindAuthorityInstance  = "realm-authority-instance"
	ResourceKindRealm              = "realm"
	ResourceKindGenerationDelivery = "realm-channel-delivery"
	ResourceKindChannel            = "realm-channel"
	ResourceKindOperation          = "realm-channel-operation"
	PrimaryAuthorityInstance       = "primary"

	PhaseUninitialized                                    = "uninitialized"
	PhaseCheckpointing                                    = "checkpointing"
	PhaseReady                                            = "ready"
	PhaseRecoveryRequired                                 = "recovery_required"
	PhaseRecoveryOnly                                     = "recovery_only"
	DeliveryPhaseIssued                                   = "issued"
	DeliveryPhaseInstalled                                = "installed"
	DeliveryPhaseDelivering                               = "delivering"
	DeliveryPhaseActivationCommitted                      = "activation_committed"
	DeliveryPhaseCompleted                                = "completed"
	MemberStateCandidate                                  = "candidate"
	MemberStateActive                                     = "active"
	MemberStateSuspended                                  = "suspended"
	MemberStateRemoved                                    = "removed"
	MembershipChangeAdd              MembershipChangeKind = "add"
	MembershipChangeRemove           MembershipChangeKind = "remove"
	ReadinessReady                                        = "ready"
	ReadinessUnavailable                                  = "unavailable"
	ReadinessDegraded                                     = "degraded"
	ReadinessRecoveryRequired                             = "recovery_required"

	ReasonNone                        = ""
	ReasonUninitialized               = "authority_uninitialized"
	ReasonStoreUnavailable            = "authority_store_unavailable"
	ReasonSignerUnavailable           = "authority_signer_unavailable"
	ReasonSignerMismatch              = "authority_signer_mismatch"
	ReasonRepositoryUnavailable       = "checkpoint_repository_unavailable"
	ReasonAuditUnavailable            = "authority_audit_unavailable"
	ReasonCheckpointMissing           = "checkpoint_head_missing"
	ReasonCheckpointMismatch          = "checkpoint_head_mismatch"
	ReasonPersistedStateInvalid       = "authority_state_invalid"
	ReasonDeliveryPending             = "authority_delivery_pending"
	ReasonChannelGrantPending         = identityapi.ChannelGrantReasonPending
	ReasonChannelGrantRenewalDue      = identityapi.ChannelGrantReasonRenewalDue
	ReasonChannelGrantExpired         = identityapi.ChannelGrantReasonExpired
	ReasonRestoreVerificationRequired = "authority_restore_verification_required"
)

var (
	ErrInvalidArgument    = errors.New("authority invalid argument")
	ErrUnsupportedVersion = errors.New("authority unsupported version")
	ErrPermissionDenied   = errors.New("authority permission denied")
	ErrConflict           = errors.New("authority conflict")
	ErrResourceExhausted  = errors.New("authority resource exhausted")
	ErrUnavailable        = errors.New("authority unavailable")
	ErrRecoveryRequired   = errors.New("authority recovery required")
	ErrCorruptState       = errors.New("authority state is corrupt")
)

var (
	realmIDPattern     = regexp.MustCompile(`^r1_[0-9a-f]{32}$`)
	operationIDPattern = regexp.MustCompile(`^rao1_[0-9a-f]{32}$`)
	auditIDPattern     = regexp.MustCompile(`^raa1_[0-9a-f]{32}$`)
	deliveryIDPattern  = regexp.MustCompile(`^rad1_[0-9a-f]{32}$`)
	digestPattern      = regexp.MustCompile(`^(ac1|aa1)_[0-9a-f]{64}$`)
)

func ValidRealmID(value string) bool { return realmIDPattern.MatchString(value) }
