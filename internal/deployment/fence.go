package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"ardents/internal/authority"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	FenceTransactionVersion               = "topology-fence-transaction/v1"
	DeploymentFenceEvidenceVersion uint32 = 1
	FenceStatusVersion                    = "ardents.topology.fence-status/v1"
	ActionTopologyNodeFence               = "topology.node.fence"

	maxFenceDuration = 30 * time.Minute

	// MaxFenceJournalBytes bounds protected journal adapters.
	MaxFenceJournalBytes = 64 << 10
)

var (
	ErrFenceJournalBinding  = errors.New("fence journal binding mismatch")
	ErrFenceJournalConflict = errors.New("fence journal revision conflict")
	ErrFenceJournalInvalid  = errors.New("fence journal is invalid")

	fenceRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	fenceOperationPattern = regexp.MustCompile(`^rao1_[0-9a-f]{32}$`)
	fenceDigestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type FencePhase string
type FenceReason string
type FenceControlKind string

const (
	FenceReasonMembershipRemoved FenceReason = "membership_removed"

	FenceControlTargetStopped        FenceControlKind = "target_stopped"
	FenceControlTargetIngressBlocked FenceControlKind = "target_ingress_blocked"
	FenceControlDiscoveryWithdrawn   FenceControlKind = "discovery_withdrawn"
	FenceControlPeerIDDenied         FenceControlKind = "peer_id_denied"
)

const (
	FencePhaseRequested           FencePhase = "requested"
	FencePhaseIsolationPending    FencePhase = "isolation_pending"
	FencePhaseEvidencePersisted   FencePhase = "evidence_persisted"
	FencePhaseAuthorityPending    FencePhase = "authority_pending"
	FencePhaseCheckpointPersisted FencePhase = "checkpoint_persisted"
	FencePhasePeersAcknowledged   FencePhase = "peers_acknowledged"
	FencePhaseFenced              FencePhase = "fenced"
	FencePhaseRecoveryRequired    FencePhase = "recovery_required"
)

type FenceOutcome string

const (
	FenceOutcomeFenced           FenceOutcome = "fenced"
	FenceOutcomeRecoveryRequired FenceOutcome = "recovery_required"
)

type FenceFailureReason string

const (
	FenceFailureClockUnavailable      FenceFailureReason = "clock_unavailable"
	FenceFailureClockSkew             FenceFailureReason = "clock_skew_exceeded"
	FenceFailureIsolationUnavailable  FenceFailureReason = "isolation_unavailable"
	FenceFailureInvalidEvidence       FenceFailureReason = "invalid_fence_evidence"
	FenceFailureAuthorityUnavailable  FenceFailureReason = "authority_unavailable"
	FenceFailureAuthorityDenied       FenceFailureReason = "authority_denied"
	FenceFailureRepositoryUnavailable FenceFailureReason = "checkpoint_repository_unavailable"
	FenceFailureCheckpointMismatch    FenceFailureReason = "checkpoint_mismatch"
	FenceFailureSurvivorUnavailable   FenceFailureReason = "survivor_unavailable"
	FenceFailureSurvivorMismatch      FenceFailureReason = "survivor_receipt_mismatch"
	FenceFailureInvalidResponse       FenceFailureReason = "invalid_response"
)

// FenceDependencyError exposes only a stable dependency failure class.
type FenceDependencyError FenceFailureReason

func (err FenceDependencyError) Error() string { return string(err) }

// FenceRequest binds one idempotent fencing transaction.
type FenceRequest struct {
	Manifest   []byte
	TargetSlot string
	Reason     FenceReason
	Actor      string
	RequestID  string
	StartedAt  time.Time
	Deadline   time.Time
}

// FenceStatus is the bounded ordinary result. String deliberately mirrors only
// this redacted projection.
type FenceStatus struct {
	APIVersion    string             `json:"api_version"`
	TargetSlot    string             `json:"target_slot"`
	Outcome       FenceOutcome       `json:"outcome"`
	Phase         FencePhase         `json:"phase"`
	Reason        FenceFailureReason `json:"reason,omitempty"`
	ControlCount  int                `json:"control_count"`
	SurvivorCount int                `json:"survivor_count"`
}

func (status FenceStatus) String() string {
	return fmt.Sprintf(
		"fence target=%s outcome=%s phase=%s reason=%s controls=%d survivors=%d",
		status.TargetSlot, status.Outcome, status.Phase, status.Reason,
		status.ControlCount, status.SurvivorCount,
	)
}

// FenceControlReceipt is protected attributable proof of one isolation
// control. ReceiptDigest is retained, but never included in FenceStatus.
type FenceControlReceipt struct {
	Kind          FenceControlKind `json:"kind"`
	Actor         string           `json:"actor"`
	ReceiptDigest string           `json:"receipt_digest"`
}

// DeploymentFenceEvidence is the protected deployment evidence consumed by
// the accepted Authority contract.
type DeploymentFenceEvidence struct {
	Version         uint32                `json:"version"`
	RealmID         string                `json:"realm_id"`
	OperationID     string                `json:"operation_id"`
	TargetPrincipal string                `json:"target_principal"`
	ManifestDigest  string                `json:"manifest_digest"`
	RequestID       string                `json:"request_id"`
	Reason          FenceReason           `json:"reason"`
	ObservedAt      time.Time             `json:"observed_at"`
	ClockSkewSecond int64                 `json:"clock_skew_seconds"`
	Controls        []FenceControlReceipt `json:"controls"`
}

// FenceTransaction is the protected durable state. Expected identities are
// hashed in its core binding; raw target/Actor values exist only where the
// accepted evidence contract requires them.
type FenceTransaction struct {
	Version                string                   `json:"version"`
	Revision               uint64                   `json:"revision"`
	ManifestDigest         string                   `json:"manifest_digest"`
	TargetSlot             string                   `json:"target_slot"`
	ExpectedPrincipalHash  string                   `json:"expected_principal_hash"`
	ExpectedWakuPeerIDHash string                   `json:"expected_waku_peer_id_hash"`
	Actor                  string                   `json:"actor"`
	RequestID              string                   `json:"request_id"`
	Reason                 FenceReason              `json:"reason"`
	StartedAt              time.Time                `json:"started_at"`
	Deadline               time.Time                `json:"deadline"`
	Phase                  FencePhase               `json:"phase"`
	ResumeFrom             FencePhase               `json:"resume_from,omitempty"`
	FailureReason          FenceFailureReason       `json:"failure_reason,omitempty"`
	OperationID            string                   `json:"operation_id,omitempty"`
	Evidence               *DeploymentFenceEvidence `json:"evidence,omitempty"`
	IsolationControls      []FenceControlReceipt    `json:"isolation_controls,omitempty"`
	ClockObservedAt        time.Time                `json:"clock_observed_at,omitempty"`
	ClockSkewSecond        int64                    `json:"clock_skew_seconds,omitempty"`
	EvidenceDigest         string                   `json:"evidence_digest,omitempty"`
	AuthorityChannelID     string                   `json:"authority_channel_id,omitempty"`
	AuthorityGeneration    uint32                   `json:"authority_generation,omitempty"`
	CheckpointDigest       string                   `json:"checkpoint_digest,omitempty"`
	RepositoryPersisted    bool                     `json:"repository_persisted,omitempty"`
	SurvivorReceipts       map[string]string        `json:"survivor_receipts,omitempty"`
}

// FenceTarget carries protected manifest bindings to consumer-owned adapters.
type FenceTarget struct {
	TargetSlot         string
	TargetPrincipal    string
	ExpectedWakuPeerID string
	SurvivorSlots      []string
	Actor              string
	RequestID          string
	Reason             FenceReason
	ManifestDigest     string
	RealmID            string
	StartedAt          time.Time
	Deadline           time.Time
}

// FenceAuthorityRequest binds the exact protected action and resource.
type FenceAuthorityRequest struct {
	Action          string
	Resource        string
	Actor           string
	Effective       string
	Delegated       bool
	RealmID         string
	TargetPrincipal string
	RequestID       string
	Reason          FenceReason
	ManifestDigest  string
	SurvivorSlots   []string
}

type FenceAuthorityResult struct {
	OperationID         string
	TargetPrincipal     string
	EvidenceDigest      string
	EvidenceAccepted    bool
	ChannelID           string
	Generation          uint32
	CheckpointDigest    string
	RepositoryPersisted bool
	SurvivorReceipts    map[string]string
}

type FenceJournalStore interface {
	Load(context.Context) (FenceTransaction, bool, error)
	Save(context.Context, uint64, FenceTransaction) error
}

type FenceClockResult struct {
	ObservedAt      time.Time
	ClockSkewSecond int64
}

type FenceIsolation interface {
	// Preflight returns the attributable bounded clock result stored in
	// DeploymentFenceEvidence.
	Preflight(context.Context, FenceTarget) (FenceClockResult, error)
	// Enforce must be idempotent for the immutable target RequestID. A retry
	// after ambiguous coordinator failure must return the same enforced truth.
	Enforce(context.Context, FenceTarget) ([]FenceControlReceipt, error)
}

type FenceAuthority interface {
	// PrepareRemoval must be idempotent for RealmID and RequestID and return the
	// same Authority-owned operation identity after ambiguous completion.
	PrepareRemoval(context.Context, FenceAuthorityRequest) (string, error)
	// CompleteRemoval consumes the same evidence idempotently.
	CompleteRemoval(context.Context, DeploymentFenceEvidence) (FenceAuthorityResult, error)
}

type FenceCoordinator struct {
	Journal   FenceJournalStore
	Isolation FenceIsolation
	Authority FenceAuthority
	Clock     func() time.Time
}

func (coordinator FenceCoordinator) Fence(
	ctx context.Context,
	request FenceRequest,
) (FenceStatus, error) {
	manifest, target, err := validateFenceRequest(request)
	if err != nil {
		return FenceStatus{}, err
	}
	if coordinator.Journal == nil || coordinator.Isolation == nil ||
		coordinator.Authority == nil || coordinator.Clock == nil {
		return FenceStatus{}, ValidationError("topology_fence_dependencies_required")
	}
	transaction, found, err := coordinator.Journal.Load(ctx)
	if err != nil {
		return FenceStatus{}, err
	}
	if !found {
		transaction, err = newFenceTransaction(request, manifest)
		if err != nil {
			return FenceStatus{}, err
		}
		if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
			return FenceStatus{}, err
		}
	} else if !sameFenceBinding(transaction, request, manifest, target) {
		return FenceStatus{}, ErrFenceJournalBinding
	}
	if err := ValidateFenceTransaction(transaction); err != nil {
		return FenceStatus{}, err
	}
	if transaction.Evidence != nil {
		if err := validateFenceEvidence(*transaction.Evidence, request, manifest, target); err != nil {
			return FenceStatus{}, ErrFenceJournalBinding
		}
	}
	if transaction.Phase == FencePhaseFenced {
		return fenceStatus(transaction), nil
	}
	if transaction.Phase == FencePhaseRecoveryRequired {
		if !resumableFencePhase(transaction.ResumeFrom) {
			return FenceStatus{}, ErrFenceJournalInvalid
		}
		transaction.Phase = transaction.ResumeFrom
		transaction.ResumeFrom = ""
		transaction.FailureReason = ""
		if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
			return FenceStatus{}, err
		}
	}
	protectedTarget := fenceTarget(request, manifest, target, transaction.ManifestDigest)
	for {
		observedAt := coordinator.Clock().UTC().Truncate(time.Second)
		if observedAt.Before(transaction.StartedAt) || observedAt.After(transaction.Deadline) {
			return coordinator.fail(
				ctx, &transaction, transaction.Phase, FenceFailureClockUnavailable,
			)
		}
		switch transaction.Phase {
		case FencePhaseRequested:
			clockResult, err := coordinator.Isolation.Preflight(ctx, protectedTarget)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, FencePhaseRequested,
					dependencyFenceReason(err, FenceFailureClockUnavailable),
				)
			}
			if !validFenceClockResult(clockResult, transaction) {
				return coordinator.fail(
					ctx, &transaction, FencePhaseRequested, FenceFailureClockSkew,
				)
			}
			transaction.ClockObservedAt = clockResult.ObservedAt.UTC()
			transaction.ClockSkewSecond = clockResult.ClockSkewSecond
			transaction.Phase = FencePhaseIsolationPending
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhaseIsolationPending:
			if len(transaction.IsolationControls) == 0 {
				controls, err := coordinator.Isolation.Enforce(ctx, protectedTarget)
				if err != nil {
					return coordinator.fail(
						ctx, &transaction, FencePhaseIsolationPending,
						dependencyFenceReason(err, FenceFailureIsolationUnavailable),
					)
				}
				if err := validateFenceControls(controls, request.Actor); err != nil {
					return coordinator.fail(
						ctx, &transaction, FencePhaseIsolationPending,
						FenceFailureInvalidEvidence,
					)
				}
				transaction.IsolationControls = append([]FenceControlReceipt(nil), controls...)
				if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
					return FenceStatus{}, err
				}
			}
			operationID, err := coordinator.Authority.PrepareRemoval(
				ctx, authorityFenceRequest(protectedTarget),
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, FencePhaseIsolationPending,
					dependencyFenceReason(err, FenceFailureAuthorityUnavailable),
				)
			}
			if !fenceOperationPattern.MatchString(operationID) {
				return coordinator.fail(ctx, &transaction, FencePhaseIsolationPending, FenceFailureInvalidResponse)
			}
			transaction.OperationID = operationID
			transaction.Evidence = &DeploymentFenceEvidence{
				Version: DeploymentFenceEvidenceVersion,
				RealmID: manifest.Authority.RealmID, OperationID: operationID,
				TargetPrincipal: target.ExpectedNodePrincipal,
				ManifestDigest:  transaction.ManifestDigest, RequestID: request.RequestID,
				Reason: request.Reason, ObservedAt: transaction.ClockObservedAt,
				ClockSkewSecond: transaction.ClockSkewSecond,
				Controls:        append([]FenceControlReceipt(nil), transaction.IsolationControls...),
			}
			if err := validateFenceEvidence(*transaction.Evidence, request, manifest, target); err != nil {
				return coordinator.fail(ctx, &transaction, FencePhaseIsolationPending, FenceFailureInvalidEvidence)
			}
			transaction.Phase = FencePhaseEvidencePersisted
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhaseEvidencePersisted:
			transaction.Phase = FencePhaseAuthorityPending
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhaseAuthorityPending:
			if transaction.Evidence == nil {
				return FenceStatus{}, ErrFenceJournalInvalid
			}
			result, err := coordinator.Authority.CompleteRemoval(ctx, *transaction.Evidence)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, FencePhaseAuthorityPending,
					dependencyFenceReason(err, FenceFailureAuthorityUnavailable),
				)
			}
			if !validFenceAuthorityResult(result, *transaction.Evidence, target.Slot, manifest) {
				reason := FenceFailureCheckpointMismatch
				if validAuthorityEvidenceAcceptance(result, *transaction.Evidence) &&
					hasDurableAuthorityCheckpoint(result) {
					reason = FenceFailureSurvivorMismatch
				}
				return coordinator.fail(ctx, &transaction, FencePhaseAuthorityPending, reason)
			}
			transaction.EvidenceDigest = result.EvidenceDigest
			transaction.AuthorityChannelID = result.ChannelID
			transaction.AuthorityGeneration = result.Generation
			transaction.CheckpointDigest = result.CheckpointDigest
			transaction.RepositoryPersisted = result.RepositoryPersisted
			transaction.SurvivorReceipts = cloneStringMap(result.SurvivorReceipts)
			transaction.Phase = FencePhaseCheckpointPersisted
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhaseCheckpointPersisted:
			if !validFenceAuthorityResult(FenceAuthorityResult{
				OperationID:         transaction.OperationID,
				TargetPrincipal:     transaction.Evidence.TargetPrincipal,
				EvidenceDigest:      transaction.EvidenceDigest,
				EvidenceAccepted:    true,
				ChannelID:           transaction.AuthorityChannelID,
				Generation:          transaction.AuthorityGeneration,
				CheckpointDigest:    transaction.CheckpointDigest,
				RepositoryPersisted: transaction.RepositoryPersisted,
				SurvivorReceipts:    transaction.SurvivorReceipts,
			}, *transaction.Evidence, target.Slot, manifest) {
				return FenceStatus{}, ErrFenceJournalInvalid
			}
			transaction.Phase = FencePhasePeersAcknowledged
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhasePeersAcknowledged:
			transaction.Phase = FencePhaseFenced
			if err := persistFence(ctx, coordinator.Journal, &transaction); err != nil {
				return FenceStatus{}, err
			}
		case FencePhaseFenced:
			return fenceStatus(transaction), nil
		default:
			return FenceStatus{}, ErrFenceJournalInvalid
		}
	}
}

func (coordinator FenceCoordinator) fail(
	ctx context.Context,
	transaction *FenceTransaction,
	resume FencePhase,
	reason FenceFailureReason,
) (FenceStatus, error) {
	transaction.Phase = FencePhaseRecoveryRequired
	transaction.ResumeFrom = resume
	transaction.FailureReason = reason
	if err := persistFence(ctx, coordinator.Journal, transaction); err != nil {
		return FenceStatus{}, err
	}
	return fenceStatus(*transaction), nil
}

func validateFenceRequest(request FenceRequest) (topologyManifest, nodeSpec, error) {
	manifest, err := decodeTopology(request.Manifest)
	if err != nil {
		return topologyManifest{}, nodeSpec{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return topologyManifest{}, nodeSpec{}, err
	}
	if !slotPattern.MatchString(request.TargetSlot) ||
		request.Reason != FenceReasonMembershipRemoved ||
		!fenceRequestIDPattern.MatchString(request.RequestID) ||
		request.StartedAt.IsZero() || request.Deadline.IsZero() ||
		request.StartedAt.Nanosecond() != 0 || request.Deadline.Nanosecond() != 0 ||
		!request.Deadline.After(request.StartedAt) ||
		request.Deadline.Sub(request.StartedAt) > maxFenceDuration {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_fence_request_invalid")
	}
	actor, err := identityprincipal.Parse(request.Actor)
	if err != nil || actor.String() != request.Actor {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_fence_actor_invalid")
	}
	for _, node := range manifest.Nodes {
		if node.Slot == request.TargetSlot {
			return manifest, node, nil
		}
	}
	return topologyManifest{}, nodeSpec{}, ValidationError("topology_fence_target_unknown")
}

func newFenceTransaction(
	request FenceRequest,
	manifest topologyManifest,
) (FenceTransaction, error) {
	var target nodeSpec
	for _, candidate := range manifest.Nodes {
		if candidate.Slot == request.TargetSlot {
			target = candidate
			break
		}
	}
	if target.Slot == "" {
		return FenceTransaction{}, ValidationError("topology_fence_target_unknown")
	}
	return FenceTransaction{
		Version: FenceTransactionVersion, ManifestDigest: canonicalFenceManifestDigest(manifest),
		TargetSlot:             request.TargetSlot,
		ExpectedPrincipalHash:  hashFenceValue([]byte(target.ExpectedNodePrincipal)),
		ExpectedWakuPeerIDHash: hashFenceValue([]byte(target.ExpectedWakuPeerID)),
		Actor:                  request.Actor, RequestID: request.RequestID, Reason: request.Reason,
		StartedAt: request.StartedAt.UTC(), Deadline: request.Deadline.UTC(),
		Phase: FencePhaseRequested,
	}, nil
}

func fenceTarget(
	request FenceRequest,
	manifest topologyManifest,
	target nodeSpec,
	manifestDigest string,
) FenceTarget {
	survivors := make([]string, 0, exactTopologyNodeCount-1)
	for _, node := range manifest.Nodes {
		if node.Slot != target.Slot {
			survivors = append(survivors, node.Slot)
		}
	}
	sort.Strings(survivors)
	return FenceTarget{
		TargetSlot: target.Slot, TargetPrincipal: target.ExpectedNodePrincipal,
		ExpectedWakuPeerID: target.ExpectedWakuPeerID, SurvivorSlots: survivors,
		Actor: request.Actor, RequestID: request.RequestID, Reason: request.Reason,
		ManifestDigest: manifestDigest, RealmID: manifest.Authority.RealmID,
		StartedAt: request.StartedAt.UTC(), Deadline: request.Deadline.UTC(),
	}
}

func authorityFenceRequest(target FenceTarget) FenceAuthorityRequest {
	return FenceAuthorityRequest{
		Action: ActionTopologyNodeFence, Resource: "node:" + target.TargetPrincipal,
		Actor: target.Actor, Effective: target.Actor, Delegated: false,
		RealmID: target.RealmID, TargetPrincipal: target.TargetPrincipal,
		RequestID: target.RequestID, Reason: target.Reason,
		ManifestDigest: target.ManifestDigest,
		SurvivorSlots:  append([]string(nil), target.SurvivorSlots...),
	}
}

func validateFenceControls(controls []FenceControlReceipt, actor string) error {
	if len(controls) < 3 || len(controls) > 4 {
		return ErrFenceJournalInvalid
	}
	required := map[FenceControlKind]bool{
		FenceControlTargetIngressBlocked: false,
		FenceControlDiscoveryWithdrawn:   false,
		FenceControlPeerIDDenied:         false,
	}
	seen := make(map[FenceControlKind]struct{}, len(controls))
	for _, control := range controls {
		if control.Actor != actor || !fenceDigestPattern.MatchString(control.ReceiptDigest) {
			return ErrFenceJournalInvalid
		}
		switch control.Kind {
		case FenceControlTargetStopped:
		case FenceControlTargetIngressBlocked, FenceControlDiscoveryWithdrawn, FenceControlPeerIDDenied:
			required[control.Kind] = true
		default:
			return ErrFenceJournalInvalid
		}
		if _, exists := seen[control.Kind]; exists {
			return ErrFenceJournalInvalid
		}
		seen[control.Kind] = struct{}{}
	}
	for _, present := range required {
		if !present {
			return ErrFenceJournalInvalid
		}
	}
	return nil
}

func sameFenceControls(left, right []FenceControlReceipt) bool {
	if len(left) != len(right) {
		return false
	}
	leftByKind := make(map[FenceControlKind]FenceControlReceipt, len(left))
	for _, control := range left {
		leftByKind[control.Kind] = control
	}
	for _, control := range right {
		if leftByKind[control.Kind] != control {
			return false
		}
	}
	return true
}

func validateFenceEvidence(
	evidence DeploymentFenceEvidence,
	request FenceRequest,
	manifest topologyManifest,
	target nodeSpec,
) error {
	if evidence.Version != DeploymentFenceEvidenceVersion ||
		evidence.RealmID != manifest.Authority.RealmID ||
		!fenceOperationPattern.MatchString(evidence.OperationID) ||
		evidence.TargetPrincipal != target.ExpectedNodePrincipal ||
		evidence.ManifestDigest != canonicalFenceManifestDigest(manifest) ||
		evidence.RequestID != request.RequestID ||
		evidence.Reason != request.Reason ||
		evidence.ObservedAt.IsZero() || evidence.ObservedAt.Nanosecond() != 0 ||
		evidence.ObservedAt.Before(request.StartedAt) ||
		evidence.ObservedAt.After(request.Deadline) ||
		evidence.ClockSkewSecond < -maxClockSkewSeconds ||
		evidence.ClockSkewSecond > maxClockSkewSeconds {
		return ErrFenceJournalInvalid
	}
	return validateFenceControls(evidence.Controls, request.Actor)
}

func hasDurableAuthorityCheckpoint(result FenceAuthorityResult) bool {
	return result.Generation > 0 &&
		fenceDigestPattern.MatchString(result.CheckpointDigest) &&
		result.RepositoryPersisted
}

func validFenceAuthorityResult(
	result FenceAuthorityResult,
	evidence DeploymentFenceEvidence,
	targetSlot string,
	manifest topologyManifest,
) bool {
	if !validAuthorityEvidenceAcceptance(result, evidence) ||
		!hasDurableAuthorityCheckpoint(result) ||
		!validFenceChannelID(result.ChannelID) ||
		len(result.SurvivorReceipts) != exactTopologyNodeCount-1 {
		return false
	}
	for _, node := range manifest.Nodes {
		if node.Slot == targetSlot {
			continue
		}
		digest, found := result.SurvivorReceipts[node.Slot]
		if !found || !fenceDigestPattern.MatchString(digest) {
			return false
		}
	}
	return true
}

func validAuthorityEvidenceAcceptance(
	result FenceAuthorityResult,
	evidence DeploymentFenceEvidence,
) bool {
	return result.OperationID == evidence.OperationID &&
		result.TargetPrincipal == evidence.TargetPrincipal &&
		result.EvidenceAccepted &&
		result.EvidenceDigest == deploymentFenceEvidenceDigest(evidence)
}

func validFenceChannelID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func deploymentFenceEvidenceDigest(evidence DeploymentFenceEvidence) string {
	controls := make([]authority.DeploymentFenceControl, 0, len(evidence.Controls))
	for _, control := range evidence.Controls {
		controls = append(controls, authority.DeploymentFenceControl{
			Kind: string(control.Kind), Actor: control.Actor,
			ReceiptDigest: control.ReceiptDigest,
		})
	}
	return authority.DeploymentFenceEvidenceDigest(authority.DeploymentFenceEvidence{
		Version: evidence.Version, RealmID: evidence.RealmID,
		OperationID: evidence.OperationID, TargetPrincipal: evidence.TargetPrincipal,
		ManifestDigest: evidence.ManifestDigest, RequestID: evidence.RequestID,
		Reason: string(evidence.Reason), ObservedAt: evidence.ObservedAt,
		ClockSkewSecond: evidence.ClockSkewSecond, Controls: controls,
	})
}

func sameFenceBinding(
	transaction FenceTransaction,
	request FenceRequest,
	manifest topologyManifest,
	target nodeSpec,
) bool {
	return transaction.Version == FenceTransactionVersion &&
		transaction.ManifestDigest == canonicalFenceManifestDigest(manifest) &&
		transaction.TargetSlot == request.TargetSlot &&
		transaction.ExpectedPrincipalHash == hashFenceValue([]byte(target.ExpectedNodePrincipal)) &&
		transaction.ExpectedWakuPeerIDHash == hashFenceValue([]byte(target.ExpectedWakuPeerID)) &&
		transaction.Actor == request.Actor && transaction.RequestID == request.RequestID &&
		transaction.Reason == request.Reason &&
		transaction.StartedAt.Equal(request.StartedAt.UTC()) &&
		transaction.Deadline.Equal(request.Deadline.UTC())
}

// ValidateFenceTransaction applies the strict protected journal schema.
func ValidateFenceTransaction(transaction FenceTransaction) error {
	if transaction.Version != FenceTransactionVersion || transaction.Revision == 0 ||
		!fenceDigestPattern.MatchString(transaction.ManifestDigest) ||
		!slotPattern.MatchString(transaction.TargetSlot) ||
		!fenceDigestPattern.MatchString(transaction.ExpectedPrincipalHash) ||
		!fenceDigestPattern.MatchString(transaction.ExpectedWakuPeerIDHash) ||
		!fenceRequestIDPattern.MatchString(transaction.RequestID) ||
		transaction.Reason != FenceReasonMembershipRemoved ||
		transaction.StartedAt.IsZero() || transaction.Deadline.IsZero() ||
		!transaction.Deadline.After(transaction.StartedAt) ||
		transaction.Deadline.Sub(transaction.StartedAt) > maxFenceDuration ||
		!validFencePhase(transaction.Phase) {
		return ErrFenceJournalInvalid
	}
	actor, err := identityprincipal.Parse(transaction.Actor)
	if err != nil || actor.String() != transaction.Actor {
		return ErrFenceJournalInvalid
	}
	if transaction.Evidence != nil {
		evidencePrincipal, principalErr := identityprincipal.Parse(
			transaction.Evidence.TargetPrincipal,
		)
		if transaction.Evidence.Version != DeploymentFenceEvidenceVersion ||
			!authority.ValidRealmID(transaction.Evidence.RealmID) ||
			principalErr != nil ||
			evidencePrincipal.String() != transaction.Evidence.TargetPrincipal ||
			hashFenceValue([]byte(transaction.Evidence.TargetPrincipal)) !=
				transaction.ExpectedPrincipalHash ||
			transaction.OperationID != transaction.Evidence.OperationID ||
			!fenceOperationPattern.MatchString(transaction.OperationID) ||
			transaction.Evidence.ManifestDigest != transaction.ManifestDigest ||
			transaction.Evidence.RequestID != transaction.RequestID ||
			transaction.Evidence.Reason != transaction.Reason ||
			!transaction.Evidence.ObservedAt.Equal(transaction.ClockObservedAt) ||
			transaction.Evidence.ClockSkewSecond != transaction.ClockSkewSecond ||
			validateFenceControls(transaction.Evidence.Controls, transaction.Actor) != nil {
			return ErrFenceJournalInvalid
		}
	}
	clockRequired := transaction.Phase != FencePhaseRequested &&
		!(transaction.Phase == FencePhaseRecoveryRequired &&
			transaction.ResumeFrom == FencePhaseRequested)
	if clockRequired && !validFenceClockResult(FenceClockResult{
		ObservedAt: transaction.ClockObservedAt, ClockSkewSecond: transaction.ClockSkewSecond,
	}, transaction) {
		return ErrFenceJournalInvalid
	}
	if len(transaction.IsolationControls) != 0 {
		if validateFenceControls(transaction.IsolationControls, transaction.Actor) != nil {
			return ErrFenceJournalInvalid
		}
	}
	if transaction.Evidence != nil &&
		!sameFenceControls(transaction.IsolationControls, transaction.Evidence.Controls) {
		return ErrFenceJournalInvalid
	}
	switch transaction.Phase {
	case FencePhaseRequested:
		if transaction.OperationID != "" || transaction.Evidence != nil ||
			len(transaction.IsolationControls) != 0 ||
			!transaction.ClockObservedAt.IsZero() || transaction.ClockSkewSecond != 0 ||
			transaction.AuthorityChannelID != "" ||
			transaction.AuthorityGeneration != 0 || transaction.CheckpointDigest != "" ||
			transaction.EvidenceDigest != "" || transaction.RepositoryPersisted ||
			len(transaction.SurvivorReceipts) != 0 {
			return ErrFenceJournalInvalid
		}
	case FencePhaseIsolationPending:
		if transaction.OperationID != "" || transaction.Evidence != nil ||
			transaction.AuthorityChannelID != "" ||
			transaction.AuthorityGeneration != 0 || transaction.CheckpointDigest != "" ||
			transaction.EvidenceDigest != "" || transaction.RepositoryPersisted ||
			len(transaction.SurvivorReceipts) != 0 {
			return ErrFenceJournalInvalid
		}
	case FencePhaseEvidencePersisted, FencePhaseAuthorityPending:
		if transaction.Evidence == nil || transaction.OperationID == "" ||
			len(transaction.IsolationControls) == 0 ||
			transaction.AuthorityChannelID != "" ||
			transaction.AuthorityGeneration != 0 || transaction.CheckpointDigest != "" ||
			transaction.EvidenceDigest != "" || transaction.RepositoryPersisted ||
			len(transaction.SurvivorReceipts) != 0 {
			return ErrFenceJournalInvalid
		}
	case FencePhaseCheckpointPersisted, FencePhasePeersAcknowledged, FencePhaseFenced:
		if transaction.Evidence == nil || transaction.OperationID == "" ||
			len(transaction.IsolationControls) == 0 ||
			!fenceDigestPattern.MatchString(transaction.EvidenceDigest) ||
			!validFenceChannelID(transaction.AuthorityChannelID) ||
			transaction.AuthorityGeneration == 0 ||
			!fenceDigestPattern.MatchString(transaction.CheckpointDigest) ||
			!transaction.RepositoryPersisted ||
			len(transaction.SurvivorReceipts) != exactTopologyNodeCount-1 {
			return ErrFenceJournalInvalid
		}
	case FencePhaseRecoveryRequired:
		if transaction.ResumeFrom == FencePhaseAuthorityPending &&
			(transaction.Evidence == nil || transaction.OperationID == "") {
			return ErrFenceJournalInvalid
		}
	}
	if transaction.Phase == FencePhaseRecoveryRequired {
		if !resumableFencePhase(transaction.ResumeFrom) ||
			!validFenceFailureReason(transaction.FailureReason) {
			return ErrFenceJournalInvalid
		}
	} else if transaction.ResumeFrom != "" || transaction.FailureReason != "" {
		return ErrFenceJournalInvalid
	}
	return nil
}

func validFencePhase(phase FencePhase) bool {
	switch phase {
	case FencePhaseRequested, FencePhaseIsolationPending, FencePhaseEvidencePersisted,
		FencePhaseAuthorityPending, FencePhaseCheckpointPersisted,
		FencePhasePeersAcknowledged, FencePhaseFenced, FencePhaseRecoveryRequired:
		return true
	default:
		return false
	}
}

func resumableFencePhase(phase FencePhase) bool {
	switch phase {
	case FencePhaseRequested, FencePhaseIsolationPending, FencePhaseAuthorityPending:
		return true
	default:
		return false
	}
}

func validFenceFailureReason(reason FenceFailureReason) bool {
	switch reason {
	case FenceFailureClockUnavailable, FenceFailureClockSkew,
		FenceFailureIsolationUnavailable, FenceFailureInvalidEvidence,
		FenceFailureAuthorityUnavailable, FenceFailureAuthorityDenied,
		FenceFailureRepositoryUnavailable, FenceFailureCheckpointMismatch,
		FenceFailureSurvivorUnavailable, FenceFailureSurvivorMismatch,
		FenceFailureInvalidResponse:
		return true
	default:
		return false
	}
}

func dependencyFenceReason(
	err error,
	fallback FenceFailureReason,
) FenceFailureReason {
	var dependency FenceDependencyError
	if errors.As(err, &dependency) {
		reason := FenceFailureReason(dependency)
		if validFenceFailureReason(reason) {
			return reason
		}
	}
	return fallback
}

func validFenceClockResult(
	result FenceClockResult,
	transaction FenceTransaction,
) bool {
	observedAt := result.ObservedAt.UTC()
	return !observedAt.IsZero() && observedAt.Nanosecond() == 0 &&
		!observedAt.Before(transaction.StartedAt) &&
		!observedAt.After(transaction.Deadline) &&
		result.ClockSkewSecond >= -maxClockSkewSeconds &&
		result.ClockSkewSecond <= maxClockSkewSeconds
}

func persistFence(
	ctx context.Context,
	store FenceJournalStore,
	transaction *FenceTransaction,
) error {
	expected := transaction.Revision
	transaction.Revision++
	if err := store.Save(ctx, expected, cloneFenceTransaction(*transaction)); err != nil {
		transaction.Revision = expected
		return err
	}
	return nil
}

func fenceStatus(transaction FenceTransaction) FenceStatus {
	outcome := FenceOutcomeRecoveryRequired
	if transaction.Phase == FencePhaseFenced {
		outcome = FenceOutcomeFenced
	}
	controlCount := 0
	if transaction.Evidence != nil {
		controlCount = len(transaction.Evidence.Controls)
	}
	return FenceStatus{
		APIVersion: FenceStatusVersion, TargetSlot: transaction.TargetSlot,
		Outcome: outcome, Phase: transaction.Phase, Reason: transaction.FailureReason,
		ControlCount: controlCount, SurvivorCount: len(transaction.SurvivorReceipts),
	}
}

func hashFenceValue(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func canonicalFenceManifestDigest(manifest topologyManifest) string {
	canonical := manifest
	canonical.SignedDNSRoots = append([]string(nil), manifest.SignedDNSRoots...)
	sort.Strings(canonical.SignedDNSRoots)
	canonical.Nodes = append([]nodeSpec(nil), manifest.Nodes...)
	for index := range canonical.Nodes {
		canonical.Nodes[index].StaticRecoveryPeers = append(
			[]string(nil),
			canonical.Nodes[index].StaticRecoveryPeers...,
		)
		sort.Strings(canonical.Nodes[index].StaticRecoveryPeers)
	}
	sort.Slice(canonical.Nodes, func(left, right int) bool {
		return canonical.Nodes[left].Slot < canonical.Nodes[right].Slot
	})
	raw, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	return hashFenceValue(raw)
}

func cloneFenceTransaction(transaction FenceTransaction) FenceTransaction {
	if transaction.Evidence != nil {
		evidence := *transaction.Evidence
		evidence.Controls = append([]FenceControlReceipt(nil), evidence.Controls...)
		transaction.Evidence = &evidence
	}
	transaction.IsolationControls = append(
		[]FenceControlReceipt(nil),
		transaction.IsolationControls...,
	)
	transaction.SurvivorReceipts = cloneStringMap(transaction.SurvivorReceipts)
	return transaction
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// SameFenceTransactionBinding reports whether two revisions describe the
// same immutable fencing request.
func SameFenceTransactionBinding(left, right FenceTransaction) bool {
	return left.Version == right.Version &&
		left.ManifestDigest == right.ManifestDigest &&
		left.TargetSlot == right.TargetSlot &&
		left.ExpectedPrincipalHash == right.ExpectedPrincipalHash &&
		left.ExpectedWakuPeerIDHash == right.ExpectedWakuPeerIDHash &&
		left.Actor == right.Actor && left.RequestID == right.RequestID &&
		left.Reason == right.Reason && left.StartedAt.Equal(right.StartedAt) &&
		left.Deadline.Equal(right.Deadline)
}

func fencePhaseOrder(phase FencePhase) int {
	switch phase {
	case FencePhaseRequested:
		return 1
	case FencePhaseIsolationPending:
		return 2
	case FencePhaseEvidencePersisted:
		return 3
	case FencePhaseAuthorityPending:
		return 4
	case FencePhaseCheckpointPersisted:
		return 5
	case FencePhasePeersAcknowledged:
		return 6
	case FencePhaseFenced:
		return 7
	case FencePhaseRecoveryRequired:
		return 8
	default:
		return 0
	}
}

// ValidFenceTransactionTransition reports whether one compare-and-save update
// is the next monotonic durable boundary.
func ValidFenceTransactionTransition(before, after FenceTransaction) bool {
	if before.Phase == FencePhaseRecoveryRequired {
		return after.Phase == before.ResumeFrom
	}
	if after.Phase == FencePhaseRecoveryRequired {
		return after.ResumeFrom == before.Phase
	}
	if before.Phase == FencePhaseIsolationPending &&
		after.Phase == FencePhaseIsolationPending {
		return len(before.IsolationControls) == 0 &&
			len(after.IsolationControls) >= 3
	}
	return fencePhaseOrder(after.Phase) == fencePhaseOrder(before.Phase)+1
}
