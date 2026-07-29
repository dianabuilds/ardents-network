package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"time"

	"ardents/internal/runtimeimage"
)

const (
	RolloutTransactionVersion = "topology-rollout-transaction/v1"
	RolloutStatusVersion      = "ardents.topology.rollout-status/v1"
	MaxRolloutJournalBytes    = 128 << 10

	RolloutPhasePreflighted       RolloutPhase = "preflighted"
	RolloutPhaseApplying          RolloutPhase = "applying"
	RolloutPhaseActivationPending RolloutPhase = "authority_activation_pending"
	RolloutPhaseActivated         RolloutPhase = "authority_activated"
	RolloutPhaseReadyToCommit     RolloutPhase = "ready_to_commit"
	RolloutPhaseCommitted         RolloutPhase = "committed"
	RolloutPhaseCompensating      RolloutPhase = "compensating"
	RolloutPhaseCompensated       RolloutPhase = "compensated"
	RolloutPhaseRecoveryRequired  RolloutPhase = "recovery_required"

	RolloutNodeMutationPending      RolloutNodePhase = "mutation_pending"
	RolloutNodeRecreated            RolloutNodePhase = "recreated"
	RolloutNodeStartPending         RolloutNodePhase = "start_pending"
	RolloutNodeStarted              RolloutNodePhase = "started"
	RolloutNodeApplied              RolloutNodePhase = "applied"
	RolloutNodeCompensating         RolloutNodePhase = "compensating"
	RolloutNodeFallbackRecreated    RolloutNodePhase = "fallback_recreated"
	RolloutNodeFallbackStartPending RolloutNodePhase = "fallback_start_pending"
	RolloutNodeFallbackStarted      RolloutNodePhase = "fallback_started"
	RolloutNodeRestored             RolloutNodePhase = "restored"
	RolloutNodeRollbackFailed       RolloutNodePhase = "rollback_failed"

	RolloutOutcomeReady            RolloutOutcome = "ready"
	RolloutOutcomeCompensated      RolloutOutcome = "compensated"
	RolloutOutcomeRecovered        RolloutOutcome = "recovered"
	RolloutOutcomeRecoveryRequired RolloutOutcome = "recovery_required"

	RolloutReasonPreflightUnavailable  RolloutFailureReason = "preflight_unavailable"
	RolloutReasonPreflightInvalid      RolloutFailureReason = "preflight_invalid"
	RolloutReasonRecreateUnavailable   RolloutFailureReason = "recreate_unavailable"
	RolloutReasonRecreateInvalid       RolloutFailureReason = "recreate_invalid"
	RolloutReasonStartUnavailable      RolloutFailureReason = "start_unavailable"
	RolloutReasonStartInvalid          RolloutFailureReason = "start_invalid"
	RolloutReasonReadinessUnavailable  RolloutFailureReason = "readiness_unavailable"
	RolloutReasonReadinessInvalid      RolloutFailureReason = "readiness_invalid"
	RolloutReasonActivationUnavailable RolloutFailureReason = "activation_unavailable"
	RolloutReasonActivationInvalid     RolloutFailureReason = "activation_invalid"
	RolloutReasonCommitUnavailable     RolloutFailureReason = "commit_unavailable"
	RolloutReasonCommitInvalid         RolloutFailureReason = "commit_invalid"
	RolloutReasonCompensationFailed    RolloutFailureReason = "compensation_failed"
	RolloutReasonDeadlineExceeded      RolloutFailureReason = "deadline_exceeded"
	RolloutReasonInterrupted           RolloutFailureReason = "interrupted_rollout"

	maxRolloutDuration         = 30 * time.Minute
	rolloutPreflightMaxAge     = 2 * time.Minute
	rolloutPreflightFutureSkew = 30 * time.Second
)

var (
	ErrRolloutJournalBinding  = errors.New("rollout journal binding mismatch")
	ErrRolloutJournalConflict = errors.New("rollout journal revision conflict")
	ErrRolloutJournalInvalid  = errors.New("rollout journal is invalid")

	rolloutVersionPattern = regexp.MustCompile(`^[0-9][0-9A-Za-z.+_-]{0,63}$`)
)

type RolloutPhase string
type RolloutNodePhase string
type RolloutOutcome string
type RolloutFailureReason string

// RolloutCompatibility is a protected release declaration, not inferred
// compatibility. Its canonical digest is retained in the journal.
type RolloutCompatibility struct {
	Kind                        AuthorityChangeKind `json:"kind"`
	FromVersion                 string              `json:"from_version"`
	ToVersion                   string              `json:"to_version"`
	MixedGenerationAllowed      bool                `json:"mixed_generation_allowed"`
	AuthorityActivationRequired bool                `json:"authority_activation_required"`
	CompleteDataRestoreRequired bool                `json:"complete_data_restore_required"`
	MaterialsPolicyDigest       string              `json:"materials_policy_digest"`
}

type RolloutRequest struct {
	Manifest       []byte
	RequestID      string
	Compatibility  RolloutCompatibility
	FallbackImages map[string]string
	StartedAt      time.Time
	Deadline       time.Time
}

type RolloutStatus struct {
	APIVersion      string               `json:"api_version"`
	Outcome         RolloutOutcome       `json:"outcome"`
	Phase           RolloutPhase         `json:"phase"`
	Reason          RolloutFailureReason `json:"reason,omitempty"`
	NodesApplied    int                  `json:"nodes_applied"`
	NodesRestored   int                  `json:"nodes_restored"`
	RecoveryPending bool                 `json:"recovery_pending"`
}

func (status RolloutStatus) String() string {
	return fmt.Sprintf(
		"rollout outcome=%s phase=%s reason=%s applied=%d restored=%d recovery_pending=%t",
		status.Outcome, status.Phase, status.Reason, status.NodesApplied,
		status.NodesRestored, status.RecoveryPending,
	)
}

type RolloutNodeTransaction struct {
	Slot          string           `json:"slot"`
	TargetImage   string           `json:"target_image"`
	FallbackImage string           `json:"fallback_image"`
	Phase         RolloutNodePhase `json:"phase"`
}

// RolloutTransaction is protected durable ordering and recovery truth.
type RolloutTransaction struct {
	Version             string                   `json:"version"`
	Revision            uint64                   `json:"revision"`
	ManifestDigest      string                   `json:"manifest_digest"`
	RequestID           string                   `json:"request_id"`
	CompatibilityDigest string                   `json:"compatibility_digest"`
	ChangeKind          AuthorityChangeKind      `json:"change_kind"`
	RestoreData         bool                     `json:"restore_data"`
	StartedAt           time.Time                `json:"started_at"`
	Deadline            time.Time                `json:"deadline"`
	Order               []string                 `json:"order"`
	Phase               RolloutPhase             `json:"phase"`
	ResumeFrom          RolloutPhase             `json:"resume_from,omitempty"`
	FailureReason       RolloutFailureReason     `json:"failure_reason,omitempty"`
	Nodes               []RolloutNodeTransaction `json:"nodes"`
	AuthorityGeneration uint32                   `json:"authority_generation,omitempty"`
	CheckpointDigest    string                   `json:"checkpoint_digest,omitempty"`
	RepositoryPersisted bool                     `json:"repository_persisted,omitempty"`
	ActiveReceiptCount  int                      `json:"active_receipt_count,omitempty"`
}

type RolloutHostTarget struct {
	ManifestDigest        string
	RequestID             string
	CompatibilityDigest   string
	Slot                  string
	Role                  string
	SSHAlias              string
	HostKeyPinRef         string
	ExpectedNodePrincipal string
	ExpectedWakuPeerID    string
	TargetImage           string
	FallbackImage         string
	Bootstrap             bool
	PersistentStore       bool
	RequiredProviderSlots []string
	Plan                  HostPlan
}

type RolloutPreflightTarget struct {
	ManifestDigest        string
	RequestID             string
	CompatibilityDigest   string
	MaterialsPolicyDigest string
	StartedAt             time.Time
	Deadline              time.Time
	Nodes                 []RolloutHostTarget
}

type RolloutNodePreflight struct {
	Slot              string
	NodePrincipal     string
	WakuPeerID        string
	Image             string
	CompositeReady    bool
	Joined            bool
	ReachabilityReady bool
	StoreReady        bool
	BackupVerified    bool
}

type RolloutPreflightObservation struct {
	ManifestDigest          string
	RequestID               string
	CompatibilityDigest     string
	ClockObservedAt         time.Time
	ClockSkewSeconds        int
	AuthorityBackupVerified bool
	RepositoryHeadDigest    string
	RepositoryHeadVerified  bool
	MaterialsPolicyDigest   string
	MaterialsVerified       bool
	Nodes                   []RolloutNodePreflight
}

type RolloutHostChange struct {
	Image        string
	Compensating bool
	RestoreData  bool
}

type RolloutHostObservation struct {
	ManifestDigest       string
	RequestID            string
	CompatibilityDigest  string
	Slot                 string
	Image                string
	IdentityPreserved    bool
	CompleteDataRestored bool
}

type RolloutReadinessObservation struct {
	ManifestDigest      string
	RequestID           string
	CompatibilityDigest string
	Slot                string
	Image               string
	NodePrincipal       string
	WakuPeerID          string
	CompositeReady      bool
	Joined              bool
	ReachabilityReady   bool
	StoreReady          bool
	ProviderSlots       []string
}

type RolloutAuthorityTarget struct {
	ManifestDigest      string
	RequestID           string
	CompatibilityDigest string
	RealmID             string
	Order               []string
}

type RolloutAuthorityObservation struct {
	ManifestDigest      string
	RequestID           string
	CompatibilityDigest string
	Activated           bool
	Generation          uint32
	CheckpointDigest    string
	RepositoryPersisted bool
	ActiveReceipts      map[string]string
}

type RolloutCommitTarget struct {
	ManifestDigest      string
	RequestID           string
	CompatibilityDigest string
	Manifest            []byte
}

type RolloutCommitObservation struct {
	ManifestDigest      string
	RequestID           string
	CompatibilityDigest string
	Committed           bool
}

type RolloutJournalStore interface {
	Load(context.Context) (RolloutTransaction, bool, error)
	Save(context.Context, uint64, RolloutTransaction) error
	Clear(context.Context, RolloutTransaction) error
}

type RolloutPreflight interface {
	Verify(context.Context, RolloutPreflightTarget) (RolloutPreflightObservation, error)
}

type RolloutHosts interface {
	Recreate(context.Context, RolloutHostTarget, RolloutHostChange) (RolloutHostObservation, error)
	Start(context.Context, RolloutHostTarget, RolloutHostChange) (RolloutHostObservation, error)
	Readiness(context.Context, RolloutHostTarget, RolloutHostChange) (RolloutReadinessObservation, error)
}

type RolloutAuthority interface {
	Status(context.Context, RolloutAuthorityTarget) (RolloutAuthorityObservation, error)
	Activate(context.Context, RolloutAuthorityTarget) (RolloutAuthorityObservation, error)
}

type RolloutCommitter interface {
	Status(context.Context, RolloutCommitTarget) (RolloutCommitObservation, error)
	Commit(context.Context, RolloutCommitTarget) (RolloutCommitObservation, error)
}

type RolloutCoordinator struct {
	Journal   RolloutJournalStore
	Preflight RolloutPreflight
	Hosts     RolloutHosts
	Authority RolloutAuthority
	Committer RolloutCommitter
	Clock     func() time.Time
}

func (coordinator RolloutCoordinator) Rollout(
	ctx context.Context,
	request RolloutRequest,
) (RolloutStatus, error) {
	manifest, targets, order, compatibilityDigest, err :=
		validateRolloutRequest(request)
	if err != nil {
		return RolloutStatus{}, err
	}
	if coordinator.Journal == nil || coordinator.Preflight == nil ||
		coordinator.Hosts == nil || coordinator.Authority == nil ||
		coordinator.Committer == nil || coordinator.Clock == nil {
		return RolloutStatus{}, ValidationError("topology_rollout_dependencies_required")
	}
	transaction, found, err := coordinator.Journal.Load(ctx)
	if err != nil {
		return RolloutStatus{}, err
	}
	if found {
		if err := ValidateRolloutTransaction(transaction); err != nil {
			return RolloutStatus{}, err
		}
		if !sameRolloutRequestBinding(
			transaction,
			request,
			manifest,
			order,
			compatibilityDigest,
		) {
			return RolloutStatus{}, ErrRolloutJournalBinding
		}
		return coordinator.resume(ctx, request, manifest, targets, &transaction)
	}

	preflightTarget := RolloutPreflightTarget{
		ManifestDigest: manifestDigest(request.Manifest),
		RequestID:      request.RequestID, CompatibilityDigest: compatibilityDigest,
		MaterialsPolicyDigest: request.Compatibility.MaterialsPolicyDigest,
		StartedAt:             request.StartedAt, Deadline: request.Deadline,
		Nodes: cloneRolloutTargets(targets),
	}
	observation, preflightErr := coordinator.Preflight.Verify(ctx, preflightTarget)
	if preflightErr != nil {
		return rolloutPreJournalFailure(RolloutReasonPreflightUnavailable), nil
	}
	if !validRolloutPreflight(
		observation,
		preflightTarget,
		manifest,
		coordinator.Clock().UTC(),
	) {
		return rolloutPreJournalFailure(RolloutReasonPreflightInvalid), nil
	}
	transaction = newRolloutTransaction(
		request,
		manifest,
		order,
		compatibilityDigest,
	)
	if err := persistRollout(ctx, coordinator.Journal, &transaction); err != nil {
		return RolloutStatus{}, err
	}
	return coordinator.apply(ctx, request, manifest, targets, &transaction)
}

func (coordinator RolloutCoordinator) apply(
	ctx context.Context,
	request RolloutRequest,
	manifest topologyManifest,
	targets []RolloutHostTarget,
	transaction *RolloutTransaction,
) (RolloutStatus, error) {
	bySlot := rolloutTargetsBySlot(targets)
	transaction.Phase = RolloutPhaseApplying
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	for _, slot := range transaction.Order {
		if !coordinator.withinDeadline(transaction) {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonDeadlineExceeded, false,
			)
		}
		target := bySlot[slot]
		change := RolloutHostChange{Image: target.TargetImage}
		transaction.Nodes = append(transaction.Nodes, RolloutNodeTransaction{
			Slot: slot, TargetImage: target.TargetImage,
			FallbackImage: target.FallbackImage,
			Phase:         RolloutNodeMutationPending,
		})
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		index := len(transaction.Nodes) - 1
		observation, err := coordinator.Hosts.Recreate(ctx, target, change)
		if err != nil {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonRecreateUnavailable, false,
			)
		}
		if !validRolloutHostObservation(observation, target, change) {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonRecreateInvalid, false,
			)
		}
		transaction.Nodes[index].Phase = RolloutNodeRecreated
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		transaction.Nodes[index].Phase = RolloutNodeStartPending
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		observation, err = coordinator.Hosts.Start(ctx, target, change)
		if err != nil {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonStartUnavailable, false,
			)
		}
		if !validRolloutHostObservation(observation, target, change) {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonStartInvalid, false,
			)
		}
		transaction.Nodes[index].Phase = RolloutNodeStarted
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		ready, err := coordinator.Hosts.Readiness(ctx, target, change)
		if err != nil {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonReadinessUnavailable, false,
			)
		}
		if !validRolloutReadiness(ready, target, change) {
			return coordinator.failAndCompensate(
				ctx, targets, transaction, RolloutReasonReadinessInvalid, false,
			)
		}
		transaction.Nodes[index].Phase = RolloutNodeApplied
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
	}

	if transaction.ChangeKind == AuthorityChangeMigration {
		transaction.Phase = RolloutPhaseActivationPending
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		target := RolloutAuthorityTarget{
			ManifestDigest:      transaction.ManifestDigest,
			RequestID:           transaction.RequestID,
			CompatibilityDigest: transaction.CompatibilityDigest,
			RealmID:             manifest.Authority.RealmID,
			Order:               append([]string(nil), transaction.Order...),
		}
		observation, err := coordinator.Authority.Activate(ctx, target)
		if err != nil ||
			!validRolloutActivation(observation, target, transaction.Order, true) {
			observed, statusErr := coordinator.Authority.Status(ctx, target)
			switch {
			case statusErr == nil &&
				validRolloutActivation(observed, target, transaction.Order, true):
				observation = observed
			case statusErr == nil &&
				validRolloutActivation(observed, target, transaction.Order, false):
				reason := RolloutReasonActivationInvalid
				if err != nil {
					reason = RolloutReasonActivationUnavailable
				}
				return coordinator.failAndCompensate(
					ctx, targets, transaction, reason, false,
				)
			default:
				return coordinator.activationRecoveryRequired(
					ctx,
					transaction,
					RolloutReasonActivationUnavailable,
				)
			}
		}
		applyRolloutActivation(transaction, observation)
		transaction.Phase = RolloutPhaseActivated
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
	}
	return coordinator.commit(ctx, request, targets, transaction)
}

func (coordinator RolloutCoordinator) commit(
	ctx context.Context,
	request RolloutRequest,
	targets []RolloutHostTarget,
	transaction *RolloutTransaction,
) (RolloutStatus, error) {
	transaction.Phase = RolloutPhaseReadyToCommit
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	target := RolloutCommitTarget{
		ManifestDigest:      transaction.ManifestDigest,
		RequestID:           transaction.RequestID,
		CompatibilityDigest: transaction.CompatibilityDigest,
		Manifest:            append([]byte(nil), request.Manifest...),
	}
	observation, commitErr := coordinator.Committer.Commit(ctx, target)
	if commitErr != nil || !validRolloutCommit(observation, target, true) {
		observed, statusErr := coordinator.Committer.Status(ctx, target)
		switch {
		case statusErr == nil && validRolloutCommit(observed, target, true):
			observation = observed
		case statusErr == nil && validRolloutCommit(observed, target, false) &&
			transaction.ChangeKind == AuthorityChangeCompatible:
			reason := RolloutReasonCommitInvalid
			if commitErr != nil {
				reason = RolloutReasonCommitUnavailable
			}
			return coordinator.failAndCompensate(
				ctx, targets, transaction, reason, false,
			)
		default:
			return coordinator.commitRecoveryRequired(
				ctx,
				transaction,
				RolloutReasonCommitUnavailable,
			)
		}
	}
	transaction.Phase = RolloutPhaseCommitted
	transaction.ResumeFrom = ""
	transaction.FailureReason = ""
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	status := rolloutStatus(*transaction, RolloutOutcomeReady)
	if err := coordinator.Journal.Clear(ctx, *transaction); err != nil {
		return RolloutStatus{}, err
	}
	return status, nil
}

func (coordinator RolloutCoordinator) resume(
	ctx context.Context,
	request RolloutRequest,
	manifest topologyManifest,
	targets []RolloutHostTarget,
	transaction *RolloutTransaction,
) (RolloutStatus, error) {
	if transaction.Phase == RolloutPhaseCommitted {
		status := rolloutStatus(*transaction, RolloutOutcomeReady)
		if err := coordinator.Journal.Clear(ctx, *transaction); err != nil {
			return RolloutStatus{}, err
		}
		return status, nil
	}
	if transaction.Phase == RolloutPhaseActivated {
		return coordinator.commit(ctx, request, targets, transaction)
	}
	if transaction.Phase == RolloutPhaseActivationPending ||
		transaction.Phase == RolloutPhaseRecoveryRequired &&
			transaction.ResumeFrom == RolloutPhaseActivationPending {
		target := RolloutAuthorityTarget{
			ManifestDigest:      transaction.ManifestDigest,
			RequestID:           transaction.RequestID,
			CompatibilityDigest: transaction.CompatibilityDigest,
			RealmID:             manifest.Authority.RealmID,
			Order:               append([]string(nil), transaction.Order...),
		}
		observation, err := coordinator.Authority.Status(ctx, target)
		switch {
		case err == nil &&
			validRolloutActivation(observation, target, transaction.Order, true):
			applyRolloutActivation(transaction, observation)
			transaction.Phase = RolloutPhaseActivated
			transaction.ResumeFrom = ""
			transaction.FailureReason = ""
			if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
				return RolloutStatus{}, err
			}
			return coordinator.commit(ctx, request, targets, transaction)
		case err == nil &&
			validRolloutActivation(observation, target, transaction.Order, false):
			transaction.FailureReason = RolloutReasonInterrupted
			return coordinator.compensate(ctx, targets, transaction, true)
		default:
			return coordinator.activationRecoveryRequired(
				ctx,
				transaction,
				RolloutReasonActivationUnavailable,
			)
		}
	}
	if transaction.Phase == RolloutPhaseReadyToCommit ||
		transaction.Phase == RolloutPhaseRecoveryRequired &&
			transaction.ResumeFrom == RolloutPhaseReadyToCommit {
		target := RolloutCommitTarget{
			ManifestDigest:      transaction.ManifestDigest,
			RequestID:           transaction.RequestID,
			CompatibilityDigest: transaction.CompatibilityDigest,
			Manifest:            append([]byte(nil), request.Manifest...),
		}
		observation, err := coordinator.Committer.Status(ctx, target)
		if err == nil && validRolloutCommit(observation, target, true) {
			transaction.Phase = RolloutPhaseCommitted
			transaction.ResumeFrom = ""
			transaction.FailureReason = ""
			if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
				return RolloutStatus{}, err
			}
			status := rolloutStatus(*transaction, RolloutOutcomeReady)
			if err := coordinator.Journal.Clear(ctx, *transaction); err != nil {
				return RolloutStatus{}, err
			}
			return status, nil
		}
		if err == nil && validRolloutCommit(observation, target, false) {
			if transaction.ChangeKind == AuthorityChangeMigration {
				transaction.Phase = RolloutPhaseActivated
				transaction.ResumeFrom = ""
				transaction.FailureReason = ""
				if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
					return RolloutStatus{}, err
				}
				return coordinator.commit(ctx, request, targets, transaction)
			}
			if transaction.FailureReason == "" {
				transaction.FailureReason = RolloutReasonInterrupted
			}
			return coordinator.compensate(ctx, targets, transaction, true)
		}
		return coordinator.commitRecoveryRequired(
			ctx,
			transaction,
			RolloutReasonCommitUnavailable,
		)
	}
	if transaction.FailureReason == "" {
		transaction.FailureReason = RolloutReasonInterrupted
	}
	return coordinator.compensate(ctx, targets, transaction, true)
}

func (coordinator RolloutCoordinator) activationRecoveryRequired(
	ctx context.Context,
	transaction *RolloutTransaction,
	reason RolloutFailureReason,
) (RolloutStatus, error) {
	transaction.Phase = RolloutPhaseRecoveryRequired
	transaction.ResumeFrom = RolloutPhaseActivationPending
	transaction.FailureReason = reason
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	return rolloutStatus(*transaction, RolloutOutcomeRecoveryRequired), nil
}

func (coordinator RolloutCoordinator) commitRecoveryRequired(
	ctx context.Context,
	transaction *RolloutTransaction,
	reason RolloutFailureReason,
) (RolloutStatus, error) {
	transaction.Phase = RolloutPhaseRecoveryRequired
	transaction.ResumeFrom = RolloutPhaseReadyToCommit
	transaction.FailureReason = reason
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	return rolloutStatus(*transaction, RolloutOutcomeRecoveryRequired), nil
}

func (coordinator RolloutCoordinator) failAndCompensate(
	ctx context.Context,
	targets []RolloutHostTarget,
	transaction *RolloutTransaction,
	reason RolloutFailureReason,
	recovered bool,
) (RolloutStatus, error) {
	transaction.Phase = RolloutPhaseCompensating
	transaction.ResumeFrom = ""
	transaction.FailureReason = reason
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	return coordinator.compensate(ctx, targets, transaction, recovered)
}

func (coordinator RolloutCoordinator) compensate(
	ctx context.Context,
	targets []RolloutHostTarget,
	transaction *RolloutTransaction,
	recovered bool,
) (RolloutStatus, error) {
	bySlot := rolloutTargetsBySlot(targets)
	if transaction.Phase != RolloutPhaseCompensating {
		transaction.Phase = RolloutPhaseCompensating
		transaction.ResumeFrom = ""
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
	}
	for index := len(transaction.Nodes) - 1; index >= 0; index-- {
		node := &transaction.Nodes[index]
		if node.Phase == RolloutNodeRestored {
			continue
		}
		target, ok := bySlot[node.Slot]
		if !ok {
			return RolloutStatus{}, ErrRolloutJournalBinding
		}
		change := RolloutHostChange{
			Image: node.FallbackImage, Compensating: true,
			RestoreData: transaction.RestoreData,
		}
		node.Phase = RolloutNodeCompensating
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		observation, err := coordinator.Hosts.Recreate(ctx, target, change)
		if err != nil || !validRolloutHostObservation(observation, target, change) {
			return coordinator.compensationFailed(ctx, transaction, node)
		}
		node.Phase = RolloutNodeFallbackRecreated
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		node.Phase = RolloutNodeFallbackStartPending
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		observation, err = coordinator.Hosts.Start(ctx, target, change)
		if err != nil || !validRolloutHostObservation(observation, target, change) {
			return coordinator.compensationFailed(ctx, transaction, node)
		}
		node.Phase = RolloutNodeFallbackStarted
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
		ready, err := coordinator.Hosts.Readiness(ctx, target, change)
		if err != nil || !validRolloutReadiness(ready, target, change) {
			return coordinator.compensationFailed(ctx, transaction, node)
		}
		node.Phase = RolloutNodeRestored
		if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
			return RolloutStatus{}, err
		}
	}
	transaction.Phase = RolloutPhaseCompensated
	transaction.ResumeFrom = ""
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	outcome := RolloutOutcomeCompensated
	if recovered {
		outcome = RolloutOutcomeRecovered
	}
	status := rolloutStatus(*transaction, outcome)
	if err := coordinator.Journal.Clear(ctx, *transaction); err != nil {
		return RolloutStatus{}, err
	}
	return status, nil
}

func (coordinator RolloutCoordinator) compensationFailed(
	ctx context.Context,
	transaction *RolloutTransaction,
	node *RolloutNodeTransaction,
) (RolloutStatus, error) {
	node.Phase = RolloutNodeRollbackFailed
	transaction.Phase = RolloutPhaseRecoveryRequired
	transaction.ResumeFrom = RolloutPhaseCompensating
	transaction.FailureReason = RolloutReasonCompensationFailed
	if err := persistRollout(ctx, coordinator.Journal, transaction); err != nil {
		return RolloutStatus{}, err
	}
	return rolloutStatus(*transaction, RolloutOutcomeRecoveryRequired), nil
}

func (coordinator RolloutCoordinator) withinDeadline(transaction *RolloutTransaction) bool {
	now := coordinator.Clock().UTC()
	return !now.Before(transaction.StartedAt) && !now.After(transaction.Deadline)
}

func validateRolloutRequest(
	request RolloutRequest,
) (topologyManifest, []RolloutHostTarget, []string, string, error) {
	manifest, err := decodeTopology(request.Manifest)
	if err != nil {
		return topologyManifest{}, nil, nil, "", err
	}
	if err := validateTopology(manifest); err != nil {
		return topologyManifest{}, nil, nil, "", err
	}
	if !fenceRequestIDPattern.MatchString(request.RequestID) ||
		request.StartedAt.IsZero() || request.Deadline.IsZero() ||
		request.StartedAt.Nanosecond() != 0 || request.Deadline.Nanosecond() != 0 ||
		!request.Deadline.After(request.StartedAt) ||
		request.Deadline.Sub(request.StartedAt) > maxRolloutDuration ||
		!validRolloutCompatibility(request.Compatibility) {
		return topologyManifest{}, nil, nil, "", ValidationError("topology_rollout_request_invalid")
	}
	if len(request.FallbackImages) != exactTopologyNodeCount {
		return topologyManifest{}, nil, nil, "", ValidationError("topology_rollout_fallbacks_invalid")
	}
	order, err := AuthorityRolloutOrder(request.Manifest, request.Compatibility.Kind)
	if err != nil {
		return topologyManifest{}, nil, nil, "", err
	}
	compatibilityDigest := rolloutCompatibilityDigest(request.Compatibility)
	targets := rolloutTargets(request, manifest, compatibilityDigest)
	for _, target := range targets {
		fallback, found := request.FallbackImages[target.Slot]
		if !found || !runtimeimage.ValidReference(fallback) ||
			fallback == target.TargetImage {
			return topologyManifest{}, nil, nil, "", ValidationError("topology_rollout_fallbacks_invalid")
		}
	}
	return manifest, targets, order, compatibilityDigest, nil
}

func validRolloutCompatibility(value RolloutCompatibility) bool {
	if !rolloutVersionPattern.MatchString(value.FromVersion) ||
		!rolloutVersionPattern.MatchString(value.ToVersion) ||
		value.FromVersion == value.ToVersion ||
		!fenceDigestPattern.MatchString(value.MaterialsPolicyDigest) {
		return false
	}
	switch value.Kind {
	case AuthorityChangeCompatible:
		return value.MixedGenerationAllowed && !value.AuthorityActivationRequired
	case AuthorityChangeMigration:
		return !value.MixedGenerationAllowed && value.AuthorityActivationRequired
	default:
		return false
	}
}

func rolloutCompatibilityDigest(value RolloutCompatibility) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("rollout compatibility contains only JSON-safe values")
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func rolloutTargets(
	request RolloutRequest,
	manifest topologyManifest,
	compatibilityDigest string,
) []RolloutHostTarget {
	plan := compilePlan(manifest)
	plans := make(map[string]HostPlan, len(plan.Hosts))
	for _, host := range plan.Hosts {
		plans[host.Slot] = host
	}
	digest := manifestDigest(request.Manifest)
	providerSlots := make([]string, 0, exactTopologyNodeCount)
	for _, node := range manifest.Nodes {
		if node.Bootstrap && node.Store.Persistent {
			providerSlots = append(providerSlots, node.Slot)
		}
	}
	sort.Strings(providerSlots)
	targets := make([]RolloutHostTarget, 0, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		role := "member"
		if node.Slot == manifest.Authority.Slot {
			role = "authority"
		}
		targets = append(targets, RolloutHostTarget{
			ManifestDigest: digest, RequestID: request.RequestID,
			CompatibilityDigest: compatibilityDigest,
			Slot:                node.Slot, Role: role,
			SSHAlias: node.Host.SSHAlias, HostKeyPinRef: node.Host.HostKeyPinRef,
			ExpectedNodePrincipal: node.ExpectedNodePrincipal,
			ExpectedWakuPeerID:    node.ExpectedWakuPeerID,
			TargetImage:           node.Image, FallbackImage: request.FallbackImages[node.Slot],
			Bootstrap: node.Bootstrap, PersistentStore: node.Store.Persistent,
			RequiredProviderSlots: append([]string(nil), providerSlots...),
			Plan:                  plans[node.Slot],
		})
	}
	sort.Slice(targets, func(left, right int) bool {
		return targets[left].Slot < targets[right].Slot
	})
	return targets
}

func validRolloutPreflight(
	value RolloutPreflightObservation,
	target RolloutPreflightTarget,
	manifest topologyManifest,
	now time.Time,
) bool {
	if value.ManifestDigest != target.ManifestDigest ||
		value.RequestID != target.RequestID ||
		value.CompatibilityDigest != target.CompatibilityDigest ||
		value.MaterialsPolicyDigest != target.MaterialsPolicyDigest ||
		!value.MaterialsVerified || !value.AuthorityBackupVerified ||
		!value.RepositoryHeadVerified ||
		!fenceDigestPattern.MatchString(value.RepositoryHeadDigest) ||
		value.ClockObservedAt.Before(target.StartedAt) ||
		value.ClockObservedAt.After(target.Deadline) ||
		value.ClockObservedAt.Before(now.Add(-rolloutPreflightMaxAge)) ||
		value.ClockObservedAt.After(now.Add(rolloutPreflightFutureSkew)) ||
		value.ClockSkewSeconds < 0 ||
		value.ClockSkewSeconds > manifest.Clock.MaxSkewSeconds ||
		len(value.Nodes) != exactTopologyNodeCount {
		return false
	}
	providers := 0
	for index, node := range value.Nodes {
		expected := target.Nodes[index]
		if node.Slot != expected.Slot ||
			node.NodePrincipal != expected.ExpectedNodePrincipal ||
			node.WakuPeerID != expected.ExpectedWakuPeerID ||
			node.Image != expected.FallbackImage ||
			!node.CompositeReady || !node.Joined || !node.ReachabilityReady ||
			!node.BackupVerified || expected.PersistentStore && !node.StoreReady {
			return false
		}
		if expected.Bootstrap && expected.PersistentStore {
			providers++
		}
	}
	return providers >= 2
}

func validRolloutHostObservation(
	value RolloutHostObservation,
	target RolloutHostTarget,
	change RolloutHostChange,
) bool {
	return value.ManifestDigest == target.ManifestDigest &&
		value.RequestID == target.RequestID &&
		value.CompatibilityDigest == target.CompatibilityDigest &&
		value.Slot == target.Slot && value.Image == change.Image &&
		value.IdentityPreserved &&
		value.CompleteDataRestored == change.RestoreData
}

func validRolloutReadiness(
	value RolloutReadinessObservation,
	target RolloutHostTarget,
	change RolloutHostChange,
) bool {
	return value.ManifestDigest == target.ManifestDigest &&
		value.RequestID == target.RequestID &&
		value.CompatibilityDigest == target.CompatibilityDigest &&
		value.Slot == target.Slot && value.Image == change.Image &&
		value.NodePrincipal == target.ExpectedNodePrincipal &&
		value.WakuPeerID == target.ExpectedWakuPeerID &&
		value.CompositeReady && value.Joined && value.ReachabilityReady &&
		(!target.PersistentStore || value.StoreReady) &&
		slices.Equal(value.ProviderSlots, target.RequiredProviderSlots)
}

func validRolloutActivation(
	value RolloutAuthorityObservation,
	target RolloutAuthorityTarget,
	order []string,
	activated bool,
) bool {
	if value.ManifestDigest != target.ManifestDigest ||
		value.RequestID != target.RequestID ||
		value.CompatibilityDigest != target.CompatibilityDigest ||
		value.Activated != activated {
		return false
	}
	if !activated {
		return value.Generation == 0 && value.CheckpointDigest == "" &&
			!value.RepositoryPersisted && len(value.ActiveReceipts) == 0
	}
	if value.Generation == 0 || !value.RepositoryPersisted ||
		!fenceDigestPattern.MatchString(value.CheckpointDigest) ||
		len(value.ActiveReceipts) != exactTopologyNodeCount {
		return false
	}
	for _, slot := range order {
		if !fenceDigestPattern.MatchString(value.ActiveReceipts[slot]) {
			return false
		}
	}
	return true
}

func applyRolloutActivation(
	transaction *RolloutTransaction,
	observation RolloutAuthorityObservation,
) {
	transaction.AuthorityGeneration = observation.Generation
	transaction.CheckpointDigest = observation.CheckpointDigest
	transaction.RepositoryPersisted = observation.RepositoryPersisted
	transaction.ActiveReceiptCount = len(observation.ActiveReceipts)
}

func validRolloutCommit(
	value RolloutCommitObservation,
	target RolloutCommitTarget,
	committed bool,
) bool {
	return value.ManifestDigest == target.ManifestDigest &&
		value.RequestID == target.RequestID &&
		value.CompatibilityDigest == target.CompatibilityDigest &&
		value.Committed == committed
}

func newRolloutTransaction(
	request RolloutRequest,
	manifest topologyManifest,
	order []string,
	compatibilityDigest string,
) RolloutTransaction {
	_ = manifest
	return RolloutTransaction{
		Version:        RolloutTransactionVersion,
		ManifestDigest: manifestDigest(request.Manifest),
		RequestID:      request.RequestID, CompatibilityDigest: compatibilityDigest,
		ChangeKind:  request.Compatibility.Kind,
		RestoreData: request.Compatibility.CompleteDataRestoreRequired,
		StartedAt:   request.StartedAt, Deadline: request.Deadline,
		Order: append([]string(nil), order...),
		Phase: RolloutPhasePreflighted,
		Nodes: []RolloutNodeTransaction{},
	}
}

func manifestDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func sameRolloutRequestBinding(
	transaction RolloutTransaction,
	request RolloutRequest,
	manifest topologyManifest,
	order []string,
	compatibilityDigest string,
) bool {
	expected := newRolloutTransaction(
		request, manifest, order, compatibilityDigest,
	)
	if !SameRolloutTransactionBinding(transaction, expected) {
		return false
	}
	targets := rolloutTargets(request, manifest, compatibilityDigest)
	for _, node := range transaction.Nodes {
		target, found := rolloutTargetsBySlot(targets)[node.Slot]
		if !found || node.TargetImage != target.TargetImage ||
			node.FallbackImage != target.FallbackImage {
			return false
		}
	}
	return true
}

func rolloutTargetsBySlot(
	targets []RolloutHostTarget,
) map[string]RolloutHostTarget {
	out := make(map[string]RolloutHostTarget, len(targets))
	for _, target := range targets {
		out[target.Slot] = target
	}
	return out
}

func persistRollout(
	ctx context.Context,
	journal RolloutJournalStore,
	transaction *RolloutTransaction,
) error {
	previous := transaction.Revision
	transaction.Revision++
	if err := ValidateRolloutTransaction(*transaction); err != nil {
		transaction.Revision = previous
		return err
	}
	if err := journal.Save(ctx, previous, cloneRolloutTransaction(*transaction)); err != nil {
		transaction.Revision = previous
		return err
	}
	return nil
}

func ValidateRolloutTransaction(value RolloutTransaction) error {
	if value.Version != RolloutTransactionVersion || value.Revision == 0 ||
		!validSHA256Digest(value.ManifestDigest) ||
		!fenceRequestIDPattern.MatchString(value.RequestID) ||
		!fenceDigestPattern.MatchString(value.CompatibilityDigest) ||
		!validRolloutTransactionKind(value) ||
		value.StartedAt.IsZero() || value.Deadline.IsZero() ||
		value.StartedAt.Nanosecond() != 0 || value.Deadline.Nanosecond() != 0 ||
		!value.Deadline.After(value.StartedAt) ||
		value.Deadline.Sub(value.StartedAt) > maxRolloutDuration ||
		len(value.Order) != exactTopologyNodeCount ||
		!validRolloutPhase(value.Phase) ||
		!validRolloutFailure(value) ||
		len(value.Nodes) > exactTopologyNodeCount {
		return ErrRolloutJournalInvalid
	}
	seen := make(map[string]struct{}, exactTopologyNodeCount)
	for _, slot := range value.Order {
		if !slotPattern.MatchString(slot) {
			return ErrRolloutJournalInvalid
		}
		if _, duplicate := seen[slot]; duplicate {
			return ErrRolloutJournalInvalid
		}
		seen[slot] = struct{}{}
	}
	for index, node := range value.Nodes {
		if index >= len(value.Order) || node.Slot != value.Order[index] ||
			!runtimeimage.ValidReference(node.TargetImage) ||
			!runtimeimage.ValidReference(node.FallbackImage) ||
			node.TargetImage == node.FallbackImage ||
			!validRolloutNodePhase(node.Phase) {
			return ErrRolloutJournalInvalid
		}
	}
	if !validRolloutPhaseState(value) {
		return ErrRolloutJournalInvalid
	}
	activationRequired := value.ChangeKind == AuthorityChangeMigration &&
		(value.Phase == RolloutPhaseActivated ||
			value.Phase == RolloutPhaseReadyToCommit ||
			value.Phase == RolloutPhaseCommitted ||
			value.Phase == RolloutPhaseRecoveryRequired &&
				value.ResumeFrom == RolloutPhaseReadyToCommit)
	if activationRequired {
		if value.AuthorityGeneration == 0 ||
			!fenceDigestPattern.MatchString(value.CheckpointDigest) ||
			!value.RepositoryPersisted ||
			value.ActiveReceiptCount != exactTopologyNodeCount {
			return ErrRolloutJournalInvalid
		}
	} else if value.AuthorityGeneration != 0 || value.CheckpointDigest != "" ||
		value.RepositoryPersisted || value.ActiveReceiptCount != 0 {
		return ErrRolloutJournalInvalid
	}
	return nil
}

func validRolloutPhaseState(value RolloutTransaction) bool {
	allApplied := len(value.Nodes) == exactTopologyNodeCount
	allRestored := len(value.Nodes) > 0
	for index, node := range value.Nodes {
		if node.Phase != RolloutNodeApplied {
			allApplied = false
		}
		if node.Phase != RolloutNodeRestored {
			allRestored = false
		}
		if value.Phase == RolloutPhaseApplying && index < len(value.Nodes)-1 &&
			node.Phase != RolloutNodeApplied {
			return false
		}
	}
	switch value.Phase {
	case RolloutPhasePreflighted:
		return len(value.Nodes) == 0
	case RolloutPhaseApplying:
		if len(value.Nodes) == 0 {
			return true
		}
		last := value.Nodes[len(value.Nodes)-1].Phase
		switch last {
		case RolloutNodeMutationPending, RolloutNodeRecreated,
			RolloutNodeStartPending, RolloutNodeStarted, RolloutNodeApplied:
			return true
		default:
			return false
		}
	case RolloutPhaseActivationPending, RolloutPhaseActivated,
		RolloutPhaseReadyToCommit, RolloutPhaseCommitted:
		return allApplied
	case RolloutPhaseCompensating:
		return len(value.Nodes) > 0
	case RolloutPhaseCompensated:
		return allRestored
	case RolloutPhaseRecoveryRequired:
		if value.ResumeFrom == RolloutPhaseReadyToCommit ||
			value.ResumeFrom == RolloutPhaseActivationPending {
			return allApplied
		}
		return len(value.Nodes) > 0
	default:
		return false
	}
}

func validRolloutTransactionKind(value RolloutTransaction) bool {
	return value.ChangeKind == AuthorityChangeCompatible ||
		value.ChangeKind == AuthorityChangeMigration
}

func validRolloutPhase(value RolloutPhase) bool {
	switch value {
	case RolloutPhasePreflighted, RolloutPhaseApplying,
		RolloutPhaseActivationPending, RolloutPhaseActivated,
		RolloutPhaseReadyToCommit, RolloutPhaseCommitted,
		RolloutPhaseCompensating, RolloutPhaseCompensated,
		RolloutPhaseRecoveryRequired:
		return true
	default:
		return false
	}
}

func validRolloutFailure(value RolloutTransaction) bool {
	if value.Phase == RolloutPhaseRecoveryRequired {
		return value.FailureReason != "" &&
			(value.ResumeFrom == RolloutPhaseCompensating ||
				value.ResumeFrom == RolloutPhaseReadyToCommit ||
				value.ResumeFrom == RolloutPhaseActivationPending)
	}
	if value.Phase == RolloutPhaseCompensating ||
		value.Phase == RolloutPhaseCompensated {
		return value.FailureReason != "" && value.ResumeFrom == ""
	}
	return value.FailureReason == "" && value.ResumeFrom == ""
}

func validRolloutNodePhase(value RolloutNodePhase) bool {
	switch value {
	case RolloutNodeMutationPending, RolloutNodeRecreated,
		RolloutNodeStartPending, RolloutNodeStarted, RolloutNodeApplied,
		RolloutNodeCompensating, RolloutNodeFallbackRecreated,
		RolloutNodeFallbackStartPending, RolloutNodeFallbackStarted,
		RolloutNodeRestored, RolloutNodeRollbackFailed:
		return true
	default:
		return false
	}
}

func SameRolloutTransactionBinding(left, right RolloutTransaction) bool {
	return left.Version == right.Version &&
		left.ManifestDigest == right.ManifestDigest &&
		left.RequestID == right.RequestID &&
		left.CompatibilityDigest == right.CompatibilityDigest &&
		left.ChangeKind == right.ChangeKind &&
		left.RestoreData == right.RestoreData &&
		left.StartedAt.Equal(right.StartedAt) &&
		left.Deadline.Equal(right.Deadline) &&
		slices.Equal(left.Order, right.Order)
}

func ValidRolloutTransactionTransition(
	before, after RolloutTransaction,
) bool {
	if after.Revision != before.Revision+1 ||
		!SameRolloutTransactionBinding(before, after) ||
		len(after.Nodes) < len(before.Nodes) ||
		len(after.Nodes) > len(before.Nodes)+1 ||
		!validRolloutPhaseTransition(before.Phase, after.Phase) {
		return false
	}
	for index := range before.Nodes {
		if before.Nodes[index].Slot != after.Nodes[index].Slot ||
			before.Nodes[index].TargetImage != after.Nodes[index].TargetImage ||
			before.Nodes[index].FallbackImage != after.Nodes[index].FallbackImage ||
			!validRolloutNodeTransition(
				before.Nodes[index].Phase,
				after.Nodes[index].Phase,
			) {
			return false
		}
	}
	return ValidateRolloutTransaction(after) == nil
}

func validRolloutPhaseTransition(before, after RolloutPhase) bool {
	if before == after {
		return true
	}
	switch before {
	case RolloutPhasePreflighted:
		return after == RolloutPhaseApplying ||
			after == RolloutPhaseCompensating
	case RolloutPhaseApplying:
		return after == RolloutPhaseActivationPending ||
			after == RolloutPhaseReadyToCommit ||
			after == RolloutPhaseCompensating
	case RolloutPhaseActivationPending:
		return after == RolloutPhaseActivated ||
			after == RolloutPhaseCompensating ||
			after == RolloutPhaseRecoveryRequired
	case RolloutPhaseActivated:
		return after == RolloutPhaseReadyToCommit ||
			after == RolloutPhaseCompensating
	case RolloutPhaseReadyToCommit:
		return after == RolloutPhaseCommitted ||
			after == RolloutPhaseCompensating ||
			after == RolloutPhaseRecoveryRequired
	case RolloutPhaseCompensating:
		return after == RolloutPhaseCompensated ||
			after == RolloutPhaseRecoveryRequired
	case RolloutPhaseRecoveryRequired:
		return after == RolloutPhaseCompensating ||
			after == RolloutPhaseRecoveryRequired ||
			after == RolloutPhaseCommitted ||
			after == RolloutPhaseActivated
	default:
		return false
	}
}

func validRolloutNodeTransition(before, after RolloutNodePhase) bool {
	if before == after {
		return true
	}
	switch before {
	case RolloutNodeMutationPending:
		return after == RolloutNodeRecreated ||
			after == RolloutNodeCompensating
	case RolloutNodeRecreated:
		return after == RolloutNodeStartPending ||
			after == RolloutNodeCompensating
	case RolloutNodeStartPending:
		return after == RolloutNodeStarted ||
			after == RolloutNodeCompensating
	case RolloutNodeStarted:
		return after == RolloutNodeApplied ||
			after == RolloutNodeCompensating
	case RolloutNodeApplied, RolloutNodeRollbackFailed:
		return after == RolloutNodeCompensating
	case RolloutNodeCompensating:
		return after == RolloutNodeFallbackRecreated ||
			after == RolloutNodeRollbackFailed
	case RolloutNodeFallbackRecreated:
		return after == RolloutNodeFallbackStartPending ||
			after == RolloutNodeRollbackFailed
	case RolloutNodeFallbackStartPending:
		return after == RolloutNodeFallbackStarted ||
			after == RolloutNodeRollbackFailed
	case RolloutNodeFallbackStarted:
		return after == RolloutNodeRestored ||
			after == RolloutNodeRollbackFailed
	case RolloutNodeRestored:
		return after == RolloutNodeRestored
	default:
		return false
	}
}

func cloneRolloutTransaction(value RolloutTransaction) RolloutTransaction {
	value.Order = append([]string(nil), value.Order...)
	value.Nodes = append([]RolloutNodeTransaction(nil), value.Nodes...)
	return value
}

func cloneRolloutTargets(value []RolloutHostTarget) []RolloutHostTarget {
	out := append([]RolloutHostTarget(nil), value...)
	for index := range out {
		out[index].Plan.StaticRecoveryPeers = append(
			[]string(nil),
			out[index].Plan.StaticRecoveryPeers...,
		)
		out[index].Plan.Checks = append([]string(nil), out[index].Plan.Checks...)
		out[index].RequiredProviderSlots = append(
			[]string(nil),
			out[index].RequiredProviderSlots...,
		)
	}
	return out
}

func rolloutPreJournalFailure(reason RolloutFailureReason) RolloutStatus {
	return RolloutStatus{
		APIVersion: RolloutStatusVersion,
		Outcome:    RolloutOutcomeRecoveryRequired,
		Phase:      RolloutPhaseRecoveryRequired, Reason: reason,
		RecoveryPending: false,
	}
}

func rolloutStatus(
	transaction RolloutTransaction,
	outcome RolloutOutcome,
) RolloutStatus {
	status := RolloutStatus{
		APIVersion: RolloutStatusVersion, Outcome: outcome,
		Phase: transaction.Phase, Reason: transaction.FailureReason,
		RecoveryPending: outcome == RolloutOutcomeRecoveryRequired,
	}
	for _, node := range transaction.Nodes {
		if node.Phase == RolloutNodeApplied {
			status.NodesApplied++
		}
		if node.Phase == RolloutNodeRestored {
			status.NodesRestored++
		}
	}
	if outcome == RolloutOutcomeReady {
		status.NodesApplied = len(transaction.Nodes)
		status.Reason = ""
	}
	return status
}
