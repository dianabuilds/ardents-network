package deployment

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"ardents/internal/authority"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	RejoinTransactionVersion = "topology-rejoin-transaction/v1"
	RejoinStatusVersion      = "ardents.topology.rejoin-status/v1"
	MaxRejoinJournalBytes    = 128 << 10
)

var (
	ErrRejoinJournalBinding  = errors.New("rejoin journal binding mismatch")
	ErrRejoinJournalConflict = errors.New("rejoin journal revision conflict")
	ErrRejoinJournalInvalid  = errors.New("rejoin journal is invalid")
)

type RejoinPhase string

const (
	RejoinPhaseRequested                    RejoinPhase = "requested"
	RejoinPhasePreflightPersisted           RejoinPhase = "preflight_persisted"
	RejoinPhaseTargetQuarantined            RejoinPhase = "target_quarantined"
	RejoinPhaseAttestationsPrepared         RejoinPhase = "attestations_prepared"
	RejoinPhaseAuthorityPending             RejoinPhase = "authority_pending"
	RejoinPhaseDeliveriesPrepared           RejoinPhase = "deliveries_prepared"
	RejoinPhaseDeliveriesInstalled          RejoinPhase = "deliveries_installed"
	RejoinPhaseActivationCommitted          RejoinPhase = "activation_committed"
	RejoinPhaseSurvivorsAcknowledged        RejoinPhase = "survivors_acknowledged"
	RejoinPhaseRestorationPending           RejoinPhase = "restoration_pending"
	RejoinPhaseReadinessVerified            RejoinPhase = "readiness_verified"
	RejoinPhaseTargetAcknowledgementPending RejoinPhase = "target_acknowledgement_pending"
	RejoinPhaseCheckpointPersisted          RejoinPhase = "checkpoint_persisted"
	RejoinPhaseRejoined                     RejoinPhase = "rejoined"
	RejoinPhaseRecoveryRequired             RejoinPhase = "recovery_required"
)

type RejoinOutcome string

const (
	RejoinOutcomeRejoined         RejoinOutcome = "rejoined"
	RejoinOutcomeRecoveryRequired RejoinOutcome = "recovery_required"
)

type RejoinFailureReason string

const (
	RejoinFailureClockUnavailable       RejoinFailureReason = "clock_unavailable"
	RejoinFailureClockSkew              RejoinFailureReason = "clock_skew_exceeded"
	RejoinFailureIsolationUnavailable   RejoinFailureReason = "isolation_unavailable"
	RejoinFailureTargetMismatch         RejoinFailureReason = "target_mismatch"
	RejoinFailureAttestationUnavailable RejoinFailureReason = "attestation_unavailable"
	RejoinFailureAttestationMismatch    RejoinFailureReason = "attestation_mismatch"
	RejoinFailureAuthorityUnavailable   RejoinFailureReason = "authority_unavailable"
	RejoinFailureAuthorityDenied        RejoinFailureReason = "authority_denied"
	RejoinFailureDeliveryUnavailable    RejoinFailureReason = "delivery_unavailable"
	RejoinFailureDeliveryMismatch       RejoinFailureReason = "delivery_mismatch"
	RejoinFailureCheckpointMismatch     RejoinFailureReason = "checkpoint_mismatch"
	RejoinFailureRepositoryUnavailable  RejoinFailureReason = "checkpoint_repository_unavailable"
	RejoinFailureSurvivorUnavailable    RejoinFailureReason = "survivor_unavailable"
	RejoinFailureSurvivorMismatch       RejoinFailureReason = "survivor_receipt_mismatch"
	RejoinFailureRestorationUnavailable RejoinFailureReason = "restoration_unavailable"
	RejoinFailureReadinessUnavailable   RejoinFailureReason = "readiness_unavailable"
	RejoinFailureReadinessMismatch      RejoinFailureReason = "readiness_mismatch"
	RejoinFailureTargetReceiptMismatch  RejoinFailureReason = "target_receipt_mismatch"
	RejoinFailureInvalidResponse        RejoinFailureReason = "invalid_response"
)

// RejoinDependencyError exposes only a stable dependency failure class.
type RejoinDependencyError RejoinFailureReason

func (err RejoinDependencyError) Error() string { return string(err) }

type RejoinRequest struct {
	Manifest   []byte
	TargetSlot string
	ChannelID  string
	Actor      string
	RequestID  string
	StartedAt  time.Time
	Deadline   time.Time
	Fence      FenceTransaction
}

type RejoinStatus struct {
	APIVersion    string              `json:"api_version"`
	TargetSlot    string              `json:"target_slot"`
	Outcome       RejoinOutcome       `json:"outcome"`
	Phase         RejoinPhase         `json:"phase"`
	Reason        RejoinFailureReason `json:"reason,omitempty"`
	DeliveryCount int                 `json:"delivery_count"`
	SurvivorCount int                 `json:"survivor_count"`
	Ready         bool                `json:"ready"`
}

func (status RejoinStatus) String() string {
	return fmt.Sprintf(
		"rejoin target=%s outcome=%s phase=%s reason=%s deliveries=%d survivors=%d ready=%t",
		status.TargetSlot, status.Outcome, status.Phase, status.Reason,
		status.DeliveryCount, status.SurvivorCount, status.Ready,
	)
}

type RejoinAttestation struct {
	RecipientPrincipal string `json:"recipient_principal"`
	Digest             string `json:"digest"`
}

type RejoinDelivery struct {
	RecipientPrincipal string `json:"recipient_principal"`
	DeliveryID         string `json:"delivery_id"`
	EnvelopeDigest     string `json:"envelope_digest"`
}

type RejoinPreparation struct {
	OperationID         string                    `json:"operation_id"`
	Generation          uint32                    `json:"generation"`
	Deliveries          map[string]RejoinDelivery `json:"deliveries"`
	CheckpointDigest    string                    `json:"checkpoint_digest"`
	RepositoryPersisted bool                      `json:"repository_persisted"`
}

type RejoinActivationResult struct {
	OperationID         string `json:"operation_id"`
	Generation          uint32 `json:"generation"`
	CheckpointDigest    string `json:"checkpoint_digest"`
	RepositoryPersisted bool   `json:"repository_persisted"`
}

type RejoinFinalResult struct {
	OperationID         string `json:"operation_id"`
	Generation          uint32 `json:"generation"`
	CheckpointDigest    string `json:"checkpoint_digest"`
	RepositoryPersisted bool   `json:"repository_persisted"`
}

type RejoinPreflightResult struct {
	HostObservedAt    map[string]time.Time
	AuthorityNotAfter time.Time
	Isolated          bool
}

type RejoinTargetObservation struct {
	Principal       string
	WakuPeerID      string
	Image           string
	ObservedAt      time.Time
	ClockSkewSecond int64
}

type RejoinReadinessResult struct {
	Principal       string
	WakuPeerID      string
	Image           string
	ObservedAt      time.Time
	ClockSkewSecond int64
	Joined          bool
	CompositeReady  bool
}

// RejoinTransaction is protected durable orchestration state. Ordinary output
// is limited to RejoinStatus.
type RejoinTransaction struct {
	Version                    string                       `json:"version"`
	Revision                   uint64                       `json:"revision"`
	ManifestDigest             string                       `json:"manifest_digest"`
	TargetSlot                 string                       `json:"target_slot"`
	ChannelID                  string                       `json:"channel_id"`
	ExpectedPrincipalHash      string                       `json:"expected_principal_hash"`
	ExpectedWakuPeerIDHash     string                       `json:"expected_waku_peer_id_hash"`
	ExpectedImageHash          string                       `json:"expected_image_hash"`
	Actor                      string                       `json:"actor"`
	RequestID                  string                       `json:"request_id"`
	StartedAt                  time.Time                    `json:"started_at"`
	Deadline                   time.Time                    `json:"deadline"`
	FenceRequestID             string                       `json:"fence_request_id"`
	FenceEvidenceDigest        string                       `json:"fence_evidence_digest"`
	RemovalOperationID         string                       `json:"removal_operation_id"`
	RemovalGeneration          uint32                       `json:"removal_generation"`
	RemovalCheckpointDigest    string                       `json:"removal_checkpoint_digest"`
	Phase                      RejoinPhase                  `json:"phase"`
	ResumeFrom                 RejoinPhase                  `json:"resume_from,omitempty"`
	FailureReason              RejoinFailureReason          `json:"failure_reason,omitempty"`
	ClockObservedAt            time.Time                    `json:"clock_observed_at,omitempty"`
	ClockSkewSecond            int64                        `json:"clock_skew_seconds,omitempty"`
	IsolationConfirmed         bool                         `json:"isolation_confirmed,omitempty"`
	TargetObserved             bool                         `json:"target_observed,omitempty"`
	Attestations               map[string]RejoinAttestation `json:"attestations,omitempty"`
	OperationID                string                       `json:"operation_id,omitempty"`
	Generation                 uint32                       `json:"generation,omitempty"`
	Deliveries                 map[string]RejoinDelivery    `json:"deliveries,omitempty"`
	DeliveryReceipts           map[string]string            `json:"delivery_receipts,omitempty"`
	PrepareCheckpointDigest    string                       `json:"prepare_checkpoint_digest,omitempty"`
	ActivationCheckpointDigest string                       `json:"activation_checkpoint_digest,omitempty"`
	RepositoryPersisted        bool                         `json:"repository_persisted,omitempty"`
	SurvivorReceipts           map[string]string            `json:"survivor_receipts,omitempty"`
	RestorationApplied         bool                         `json:"restoration_applied,omitempty"`
	ReadinessVerified          bool                         `json:"readiness_verified,omitempty"`
	TargetReceipt              string                       `json:"target_receipt,omitempty"`
	FinalCheckpointDigest      string                       `json:"final_checkpoint_digest,omitempty"`
}

type RejoinTarget struct {
	ManifestDigest          string
	RealmID                 string
	TargetSlot              string
	ChannelID               string
	TargetPrincipal         string
	ExpectedWakuPeerID      string
	ExpectedImage           string
	RecipientPrincipals     []string
	SurvivorSlots           []string
	Actor                   string
	RequestID               string
	StartedAt               time.Time
	Deadline                time.Time
	FenceRequestID          string
	FenceEvidenceDigest     string
	RemovalOperationID      string
	RemovalGeneration       uint32
	RemovalCheckpointDigest string
}

type RejoinJournalStore interface {
	Load(context.Context) (RejoinTransaction, bool, error)
	Save(context.Context, uint64, RejoinTransaction) error
}

type RejoinRestoration interface {
	Preflight(context.Context, RejoinTarget) (RejoinPreflightResult, error)
	StartQuarantined(context.Context, RejoinTarget) (RejoinTargetObservation, error)
	Restore(context.Context, RejoinTarget) error
	Reisolate(context.Context, RejoinTarget) error
}

type RejoinMembers interface {
	Attest(context.Context, RejoinTarget) (map[string]RejoinAttestation, error)
	InstallPending(
		context.Context,
		RejoinTarget,
		map[string]RejoinDelivery,
	) (map[string]string, error)
	ActivateSurvivors(context.Context, RejoinTarget, string, uint32) (map[string]string, error)
	ActivateTarget(context.Context, RejoinTarget, string, uint32) (string, error)
}

type RejoinAuthority interface {
	PrepareAdd(
		context.Context,
		RejoinTarget,
		map[string]RejoinAttestation,
	) (RejoinPreparation, error)
	CommitActivation(
		context.Context,
		RejoinTarget,
		string,
		uint32,
	) (RejoinActivationResult, error)
	AcknowledgeSurvivors(
		context.Context,
		RejoinTarget,
		string,
		uint32,
		map[string]string,
	) error
	CompleteTarget(
		context.Context,
		RejoinTarget,
		string,
		uint32,
		string,
	) (RejoinFinalResult, error)
}

type RejoinReadiness interface {
	Verify(context.Context, RejoinTarget, uint32) (RejoinReadinessResult, error)
}

type RejoinAuthorizationBinding struct {
	Action             string
	ResourceKind       string
	ResourceID         string
	RecipientPrincipal string
}

type RejoinCoordinator struct {
	Journal     RejoinJournalStore
	Restoration RejoinRestoration
	Members     RejoinMembers
	Authority   RejoinAuthority
	Readiness   RejoinReadiness
	Clock       func() time.Time
}

func (coordinator RejoinCoordinator) Rejoin(
	ctx context.Context,
	request RejoinRequest,
) (RejoinStatus, error) {
	manifest, targetNode, err := validateRejoinRequest(request)
	if err != nil {
		return RejoinStatus{}, err
	}
	if coordinator.Journal == nil || coordinator.Restoration == nil ||
		coordinator.Members == nil || coordinator.Authority == nil ||
		coordinator.Readiness == nil || coordinator.Clock == nil {
		return RejoinStatus{}, ValidationError("topology_rejoin_dependencies_required")
	}
	target := rejoinTarget(request, manifest, targetNode)
	transaction, found, err := coordinator.Journal.Load(ctx)
	if err != nil {
		return RejoinStatus{}, err
	}
	if !found {
		transaction = newRejoinTransaction(request, manifest, targetNode)
		if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
			return RejoinStatus{}, err
		}
	} else if !sameRejoinBinding(transaction, request, manifest, targetNode) {
		return RejoinStatus{}, ErrRejoinJournalBinding
	}
	if err := ValidateRejoinTransaction(transaction); err != nil {
		return RejoinStatus{}, err
	}
	if !validRejoinPersistedState(transaction, target) {
		return RejoinStatus{}, ErrRejoinJournalBinding
	}
	if transaction.Phase == RejoinPhaseRejoined {
		return rejoinStatus(transaction), nil
	}
	if found && transaction.Phase == RejoinPhaseTargetAcknowledgementPending {
		return coordinator.fail(
			ctx, &transaction, target, RejoinPhaseRestorationPending,
			RejoinFailureTargetReceiptMismatch,
		)
	}
	if transaction.Phase == RejoinPhaseRecoveryRequired {
		if !resumableRejoinPhase(transaction.ResumeFrom) {
			return RejoinStatus{}, ErrRejoinJournalInvalid
		}
		if err := coordinator.Restoration.Reisolate(ctx, target); err != nil {
			transaction.IsolationConfirmed = false
			transaction.FailureReason = dependencyRejoinReason(
				err, RejoinFailureIsolationUnavailable,
			)
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}
			return rejoinStatus(transaction), nil
		}
		transaction.IsolationConfirmed = true
		if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
			return RejoinStatus{}, err
		}
		transaction.Phase = transaction.ResumeFrom
		transaction.ResumeFrom = ""
		transaction.FailureReason = ""
		if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
			return RejoinStatus{}, err
		}
	}

	for {
		now := coordinator.Clock().UTC().Truncate(time.Second)
		if now.Before(transaction.StartedAt) || now.After(transaction.Deadline) {
			return coordinator.fail(
				ctx, &transaction, target, transaction.Phase,
				RejoinFailureClockUnavailable,
			)
		}
		switch transaction.Phase {
		case RejoinPhaseRequested:
			result, err := coordinator.Restoration.Preflight(ctx, target)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRequested,
					dependencyRejoinReason(err, RejoinFailureIsolationUnavailable),
				)
			}
			observedAt, clockSkew, valid := validRejoinPreflight(
				result, request, manifest,
			)
			if !valid {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRequested,
					RejoinFailureClockSkew,
				)
			}
			transaction.ClockObservedAt = observedAt
			transaction.ClockSkewSecond = clockSkew
			transaction.IsolationConfirmed = true
			transaction.Phase = RejoinPhasePreflightPersisted
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhasePreflightPersisted:
			observation, err := coordinator.Restoration.StartQuarantined(ctx, target)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhasePreflightPersisted,
					dependencyRejoinReason(err, RejoinFailureIsolationUnavailable),
				)
			}
			if !validRejoinTargetObservation(observation, target, manifest) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhasePreflightPersisted,
					RejoinFailureTargetMismatch,
				)
			}
			transaction.TargetObserved = true
			transaction.Phase = RejoinPhaseTargetQuarantined
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseTargetQuarantined:
			attestations, err := coordinator.Members.Attest(ctx, target)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseTargetQuarantined,
					dependencyRejoinReason(err, RejoinFailureAttestationUnavailable),
				)
			}
			if !validRejoinAttestations(attestations, target) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseTargetQuarantined,
					RejoinFailureAttestationMismatch,
				)
			}
			transaction.Attestations = cloneRejoinAttestations(attestations)
			transaction.Phase = RejoinPhaseAttestationsPrepared
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseAttestationsPrepared:
			transaction.Phase = RejoinPhaseAuthorityPending
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseAuthorityPending:
			preparation, err := coordinator.Authority.PrepareAdd(
				ctx, target, cloneRejoinAttestations(transaction.Attestations),
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseAuthorityPending,
					dependencyRejoinReason(err, RejoinFailureAuthorityUnavailable),
				)
			}
			if !validRejoinPreparation(preparation, target) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseAuthorityPending,
					RejoinFailureDeliveryMismatch,
				)
			}
			transaction.OperationID = preparation.OperationID
			transaction.Generation = preparation.Generation
			transaction.Deliveries = cloneRejoinDeliveries(preparation.Deliveries)
			transaction.PrepareCheckpointDigest = preparation.CheckpointDigest
			transaction.RepositoryPersisted = preparation.RepositoryPersisted
			transaction.Phase = RejoinPhaseDeliveriesPrepared
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseDeliveriesPrepared:
			receipts, err := coordinator.Members.InstallPending(
				ctx, target, cloneRejoinDeliveries(transaction.Deliveries),
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseDeliveriesPrepared,
					dependencyRejoinReason(err, RejoinFailureDeliveryUnavailable),
				)
			}
			if !validRecipientDigests(receipts, target.RecipientPrincipals) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseDeliveriesPrepared,
					RejoinFailureDeliveryMismatch,
				)
			}
			transaction.DeliveryReceipts = cloneStringMap(receipts)
			transaction.Phase = RejoinPhaseDeliveriesInstalled
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseDeliveriesInstalled:
			result, err := coordinator.Authority.CommitActivation(
				ctx, target, transaction.OperationID, transaction.Generation,
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseDeliveriesInstalled,
					dependencyRejoinReason(err, RejoinFailureAuthorityUnavailable),
				)
			}
			if !validRejoinActivation(result, transaction) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseDeliveriesInstalled,
					RejoinFailureCheckpointMismatch,
				)
			}
			transaction.ActivationCheckpointDigest = result.CheckpointDigest
			transaction.RepositoryPersisted = result.RepositoryPersisted
			transaction.Phase = RejoinPhaseActivationCommitted
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseActivationCommitted:
			receipts, err := coordinator.Members.ActivateSurvivors(
				ctx, target, transaction.OperationID, transaction.Generation,
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseActivationCommitted,
					dependencyRejoinReason(err, RejoinFailureSurvivorUnavailable),
				)
			}
			if !validRecipientDigests(receipts, target.SurvivorSlots) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseActivationCommitted,
					RejoinFailureSurvivorMismatch,
				)
			}
			if err := coordinator.Authority.AcknowledgeSurvivors(
				ctx, target, transaction.OperationID, transaction.Generation,
				cloneStringMap(receipts),
			); err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseActivationCommitted,
					dependencyRejoinReason(err, RejoinFailureSurvivorUnavailable),
				)
			}
			transaction.SurvivorReceipts = cloneStringMap(receipts)
			transaction.Phase = RejoinPhaseSurvivorsAcknowledged
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseSurvivorsAcknowledged:
			transaction.Phase = RejoinPhaseRestorationPending
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseRestorationPending:
			if !transaction.RestorationApplied {
				if err := coordinator.Restoration.Restore(ctx, target); err != nil {
					return coordinator.fail(
						ctx, &transaction, target, RejoinPhaseRestorationPending,
						dependencyRejoinReason(err, RejoinFailureRestorationUnavailable),
					)
				}
				transaction.RestorationApplied = true
				transaction.IsolationConfirmed = false
				if err := coordinator.persistAfterEffect(
					ctx, &transaction, target,
				); err != nil {
					return RejoinStatus{}, err
				}
			}
			result, err := coordinator.Readiness.Verify(
				ctx, target, transaction.Generation,
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					dependencyRejoinReason(err, RejoinFailureReadinessUnavailable),
				)
			}
			if !validRejoinReadiness(result, target, manifest) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					RejoinFailureReadinessMismatch,
				)
			}
			transaction.ReadinessVerified = true
			transaction.Phase = RejoinPhaseReadinessVerified
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseReadinessVerified:
			transaction.Phase = RejoinPhaseTargetAcknowledgementPending
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseTargetAcknowledgementPending:
			receipt, err := coordinator.Members.ActivateTarget(
				ctx, target, transaction.OperationID, transaction.Generation,
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					dependencyRejoinReason(err, RejoinFailureReadinessUnavailable),
				)
			}
			if !fenceDigestPattern.MatchString(receipt) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					RejoinFailureTargetReceiptMismatch,
				)
			}
			result, err := coordinator.Authority.CompleteTarget(
				ctx, target, transaction.OperationID, transaction.Generation, receipt,
			)
			if err != nil {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					dependencyRejoinReason(err, RejoinFailureAuthorityUnavailable),
				)
			}
			if !validRejoinFinal(result, transaction) {
				return coordinator.fail(
					ctx, &transaction, target, RejoinPhaseRestorationPending,
					RejoinFailureCheckpointMismatch,
				)
			}
			transaction.TargetReceipt = receipt
			transaction.FinalCheckpointDigest = result.CheckpointDigest
			transaction.RepositoryPersisted = result.RepositoryPersisted
			transaction.Phase = RejoinPhaseCheckpointPersisted
			if err := coordinator.persistAfterEffect(
				ctx, &transaction, target,
			); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseCheckpointPersisted:
			transaction.Phase = RejoinPhaseRejoined
			if err := persistRejoin(ctx, coordinator.Journal, &transaction); err != nil {
				return RejoinStatus{}, err
			}

		case RejoinPhaseRejoined:
			return rejoinStatus(transaction), nil

		default:
			return RejoinStatus{}, ErrRejoinJournalInvalid
		}
	}
}

func (coordinator RejoinCoordinator) fail(
	ctx context.Context,
	transaction *RejoinTransaction,
	target RejoinTarget,
	resume RejoinPhase,
	reason RejoinFailureReason,
) (RejoinStatus, error) {
	isolationConfirmed := false
	if rejoinPhaseOrder(resume) >= rejoinPhaseOrder(RejoinPhasePreflightPersisted) {
		if err := coordinator.Restoration.Reisolate(ctx, target); err != nil {
			reason = dependencyRejoinReason(err, RejoinFailureIsolationUnavailable)
		} else {
			isolationConfirmed = true
		}
	}
	transaction.Phase = RejoinPhaseRecoveryRequired
	transaction.ResumeFrom = resume
	transaction.FailureReason = reason
	transaction.IsolationConfirmed = isolationConfirmed
	transaction.RestorationApplied = false
	transaction.ReadinessVerified = false
	if err := persistRejoin(ctx, coordinator.Journal, transaction); err != nil {
		return RejoinStatus{}, err
	}
	return rejoinStatus(*transaction), nil
}

func (coordinator RejoinCoordinator) persistAfterEffect(
	ctx context.Context,
	transaction *RejoinTransaction,
	target RejoinTarget,
) error {
	if err := persistRejoin(ctx, coordinator.Journal, transaction); err != nil {
		_ = coordinator.Restoration.Reisolate(ctx, target)
		return err
	}
	return nil
}

func validateRejoinRequest(
	request RejoinRequest,
) (topologyManifest, nodeSpec, error) {
	manifest, err := decodeTopology(request.Manifest)
	if err != nil {
		return topologyManifest{}, nodeSpec{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return topologyManifest{}, nodeSpec{}, err
	}
	if !slotPattern.MatchString(request.TargetSlot) ||
		len(request.ChannelID) != 32 ||
		!fenceRequestIDPattern.MatchString(request.RequestID) ||
		request.StartedAt.IsZero() || request.Deadline.IsZero() ||
		request.StartedAt.Nanosecond() != 0 || request.Deadline.Nanosecond() != 0 ||
		!request.Deadline.After(request.StartedAt) ||
		request.Deadline.Sub(request.StartedAt) > maxFenceDuration {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_rejoin_request_invalid")
	}
	if _, err := hex.DecodeString(request.ChannelID); err != nil {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_rejoin_channel_invalid")
	}
	actor, err := identityprincipal.Parse(request.Actor)
	if err != nil || actor.String() != request.Actor {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_rejoin_actor_invalid")
	}
	var target nodeSpec
	for _, node := range manifest.Nodes {
		if node.Slot == request.TargetSlot {
			target = node
			break
		}
	}
	if target.Slot == "" {
		return topologyManifest{}, nodeSpec{}, ValidationError("topology_rejoin_target_unknown")
	}
	if err := ValidateFenceTransaction(request.Fence); err != nil ||
		request.Fence.Phase != FencePhaseFenced ||
		request.Fence.TargetSlot != request.TargetSlot ||
		request.Fence.ManifestDigest != canonicalFenceManifestDigest(manifest) ||
		request.Fence.AuthorityChannelID != request.ChannelID ||
		request.Fence.ExpectedPrincipalHash != hashFenceValue([]byte(target.ExpectedNodePrincipal)) ||
		request.Fence.ExpectedWakuPeerIDHash != hashFenceValue([]byte(target.ExpectedWakuPeerID)) ||
		request.Fence.OperationID == "" || request.Fence.EvidenceDigest == "" ||
		request.Fence.AuthorityGeneration == 0 ||
		request.Fence.CheckpointDigest == "" ||
		!request.Fence.RepositoryPersisted ||
		len(request.Fence.SurvivorReceipts) != 2 {
		return topologyManifest{}, nodeSpec{}, ErrRejoinJournalBinding
	}
	return manifest, target, nil
}

func newRejoinTransaction(
	request RejoinRequest,
	manifest topologyManifest,
	target nodeSpec,
) RejoinTransaction {
	return RejoinTransaction{
		Version: RejoinTransactionVersion, ManifestDigest: canonicalFenceManifestDigest(manifest),
		TargetSlot:             request.TargetSlot,
		ChannelID:              request.ChannelID,
		ExpectedPrincipalHash:  hashFenceValue([]byte(target.ExpectedNodePrincipal)),
		ExpectedWakuPeerIDHash: hashFenceValue([]byte(target.ExpectedWakuPeerID)),
		ExpectedImageHash:      hashFenceValue([]byte(target.Image)),
		Actor:                  request.Actor, RequestID: request.RequestID,
		StartedAt: request.StartedAt.UTC(), Deadline: request.Deadline.UTC(),
		FenceRequestID:          request.Fence.RequestID,
		FenceEvidenceDigest:     request.Fence.EvidenceDigest,
		RemovalOperationID:      request.Fence.OperationID,
		RemovalGeneration:       request.Fence.AuthorityGeneration,
		RemovalCheckpointDigest: request.Fence.CheckpointDigest,
		Phase:                   RejoinPhaseRequested,
	}
}

func rejoinTarget(
	request RejoinRequest,
	manifest topologyManifest,
	target nodeSpec,
) RejoinTarget {
	recipients := make([]string, 0, len(manifest.Nodes))
	survivors := make([]string, 0, len(manifest.Nodes)-1)
	for _, node := range manifest.Nodes {
		recipients = append(recipients, node.ExpectedNodePrincipal)
		if node.Slot != target.Slot {
			survivors = append(survivors, node.Slot)
		}
	}
	sort.Strings(recipients)
	sort.Strings(survivors)
	return RejoinTarget{
		ManifestDigest: canonicalFenceManifestDigest(manifest),
		RealmID:        manifest.Authority.RealmID, TargetSlot: target.Slot,
		ChannelID:          request.ChannelID,
		TargetPrincipal:    target.ExpectedNodePrincipal,
		ExpectedWakuPeerID: target.ExpectedWakuPeerID, ExpectedImage: target.Image,
		RecipientPrincipals: recipients, SurvivorSlots: survivors,
		Actor: request.Actor, RequestID: request.RequestID,
		StartedAt: request.StartedAt.UTC(), Deadline: request.Deadline.UTC(),
		FenceRequestID:          request.Fence.RequestID,
		FenceEvidenceDigest:     request.Fence.EvidenceDigest,
		RemovalOperationID:      request.Fence.OperationID,
		RemovalGeneration:       request.Fence.AuthorityGeneration,
		RemovalCheckpointDigest: request.Fence.CheckpointDigest,
	}
}

func validRejoinPreflight(
	result RejoinPreflightResult,
	request RejoinRequest,
	manifest topologyManifest,
) (time.Time, int64, bool) {
	if !result.Isolated || len(result.HostObservedAt) != len(manifest.Nodes) ||
		result.AuthorityNotAfter.IsZero() {
		return time.Time{}, 0, false
	}
	var earliest, latest time.Time
	for _, node := range manifest.Nodes {
		observedAt, ok := result.HostObservedAt[node.Slot]
		observedAt = observedAt.UTC()
		if !ok || observedAt.IsZero() || observedAt.Nanosecond() != 0 ||
			observedAt.Before(request.StartedAt) ||
			observedAt.After(request.Deadline) {
			return time.Time{}, 0, false
		}
		if earliest.IsZero() || observedAt.Before(earliest) {
			earliest = observedAt
		}
		if latest.IsZero() || observedAt.After(latest) {
			latest = observedAt
		}
	}
	skew := int64(latest.Sub(earliest) / time.Second)
	margin := time.Duration(
		manifest.Clock.AuthoritySafetyMarginSeconds,
	) * time.Second
	if skew > int64(manifest.Clock.MaxSkewSeconds) ||
		result.AuthorityNotAfter.UTC().Before(latest.Add(margin)) {
		return time.Time{}, 0, false
	}
	return latest, skew, true
}

func validRejoinTargetObservation(
	result RejoinTargetObservation,
	target RejoinTarget,
	manifest topologyManifest,
) bool {
	skew := result.ClockSkewSecond
	if skew < 0 {
		skew = -skew
	}
	return result.Principal == target.TargetPrincipal &&
		result.WakuPeerID == target.ExpectedWakuPeerID &&
		result.Image == target.ExpectedImage &&
		!result.ObservedAt.Before(target.StartedAt) &&
		!result.ObservedAt.After(target.Deadline) &&
		skew <= int64(manifest.Clock.MaxSkewSeconds)
}

func validRejoinReadiness(
	result RejoinReadinessResult,
	target RejoinTarget,
	manifest topologyManifest,
) bool {
	return result.Joined && result.CompositeReady &&
		validRejoinTargetObservation(RejoinTargetObservation{
			Principal: result.Principal, WakuPeerID: result.WakuPeerID,
			Image: result.Image, ObservedAt: result.ObservedAt,
			ClockSkewSecond: result.ClockSkewSecond,
		}, target, manifest)
}

func validRejoinAttestations(
	values map[string]RejoinAttestation,
	target RejoinTarget,
) bool {
	if len(values) != len(target.RecipientPrincipals) {
		return false
	}
	for _, principal := range target.RecipientPrincipals {
		value, ok := values[principal]
		if !ok || value.RecipientPrincipal != principal ||
			!fenceDigestPattern.MatchString(value.Digest) {
			return false
		}
	}
	return true
}

func validRejoinPreparation(
	value RejoinPreparation,
	target RejoinTarget,
) bool {
	if !fenceOperationPattern.MatchString(value.OperationID) ||
		value.Generation <= target.RemovalGeneration ||
		!fenceDigestPattern.MatchString(value.CheckpointDigest) ||
		!value.RepositoryPersisted ||
		len(value.Deliveries) != len(target.RecipientPrincipals) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Deliveries))
	for _, principal := range target.RecipientPrincipals {
		delivery, ok := value.Deliveries[principal]
		if !ok || delivery.RecipientPrincipal != principal ||
			!fenceRequestIDPattern.MatchString(delivery.DeliveryID) ||
			!fenceDigestPattern.MatchString(delivery.EnvelopeDigest) {
			return false
		}
		if _, duplicate := seen[delivery.DeliveryID]; duplicate {
			return false
		}
		seen[delivery.DeliveryID] = struct{}{}
	}
	return true
}

func validRejoinActivation(
	value RejoinActivationResult,
	transaction RejoinTransaction,
) bool {
	return value.OperationID == transaction.OperationID &&
		value.Generation == transaction.Generation &&
		fenceDigestPattern.MatchString(value.CheckpointDigest) &&
		value.RepositoryPersisted
}

func validRejoinFinal(
	value RejoinFinalResult,
	transaction RejoinTransaction,
) bool {
	return value.OperationID == transaction.OperationID &&
		value.Generation == transaction.Generation &&
		fenceDigestPattern.MatchString(value.CheckpointDigest) &&
		value.RepositoryPersisted
}

func validRecipientDigests(values map[string]string, expected []string) bool {
	if len(values) != len(expected) {
		return false
	}
	for _, key := range expected {
		if !fenceDigestPattern.MatchString(values[key]) {
			return false
		}
	}
	return true
}

func validRejoinPersistedState(
	transaction RejoinTransaction,
	target RejoinTarget,
) bool {
	if transaction.Attestations != nil &&
		!validRejoinAttestations(transaction.Attestations, target) {
		return false
	}
	if transaction.Deliveries != nil &&
		!validRejoinPreparation(RejoinPreparation{
			OperationID: transaction.OperationID, Generation: transaction.Generation,
			Deliveries:          transaction.Deliveries,
			CheckpointDigest:    transaction.PrepareCheckpointDigest,
			RepositoryPersisted: transaction.RepositoryPersisted,
		}, target) {
		return false
	}
	if transaction.DeliveryReceipts != nil &&
		!validRecipientDigests(
			transaction.DeliveryReceipts, target.RecipientPrincipals,
		) {
		return false
	}
	if transaction.SurvivorReceipts != nil &&
		!validRecipientDigests(transaction.SurvivorReceipts, target.SurvivorSlots) {
		return false
	}
	return transaction.TargetReceipt == "" ||
		fenceDigestPattern.MatchString(transaction.TargetReceipt)
}

// ValidateRejoinTransaction rejects unknown, incomplete, or contradictory
// protected state before any dependency is called.
func ValidateRejoinTransaction(transaction RejoinTransaction) error {
	if transaction.Version != RejoinTransactionVersion ||
		transaction.Revision == 0 ||
		!fenceDigestPattern.MatchString(transaction.ManifestDigest) ||
		!slotPattern.MatchString(transaction.TargetSlot) ||
		len(transaction.ChannelID) != 32 ||
		!fenceDigestPattern.MatchString(transaction.ExpectedPrincipalHash) ||
		!fenceDigestPattern.MatchString(transaction.ExpectedWakuPeerIDHash) ||
		!fenceDigestPattern.MatchString(transaction.ExpectedImageHash) ||
		!fenceRequestIDPattern.MatchString(transaction.RequestID) ||
		transaction.StartedAt.IsZero() || transaction.Deadline.IsZero() ||
		!transaction.Deadline.After(transaction.StartedAt) ||
		transaction.Deadline.Sub(transaction.StartedAt) > maxFenceDuration ||
		!fenceRequestIDPattern.MatchString(transaction.FenceRequestID) ||
		!fenceDigestPattern.MatchString(transaction.FenceEvidenceDigest) ||
		!fenceOperationPattern.MatchString(transaction.RemovalOperationID) ||
		transaction.RemovalGeneration == 0 ||
		!fenceDigestPattern.MatchString(transaction.RemovalCheckpointDigest) ||
		rejoinPhaseOrder(transaction.Phase) == 0 {
		return ErrRejoinJournalInvalid
	}
	if _, err := hex.DecodeString(transaction.ChannelID); err != nil {
		return ErrRejoinJournalInvalid
	}
	if _, err := identityprincipal.Parse(transaction.Actor); err != nil {
		return ErrRejoinJournalInvalid
	}
	if transaction.Phase == RejoinPhaseRecoveryRequired {
		if !resumableRejoinPhase(transaction.ResumeFrom) ||
			transaction.FailureReason == "" {
			return ErrRejoinJournalInvalid
		}
	} else if transaction.ResumeFrom != "" || transaction.FailureReason != "" {
		return ErrRejoinJournalInvalid
	}
	order := rejoinPhaseOrder(transaction.Phase)
	if transaction.Phase == RejoinPhaseRecoveryRequired {
		order = rejoinPhaseOrder(transaction.ResumeFrom)
	}
	if order >= rejoinPhaseOrder(RejoinPhasePreflightPersisted) &&
		transaction.ClockObservedAt.IsZero() {
		return ErrRejoinJournalInvalid
	}
	if transaction.Phase != RejoinPhaseRecoveryRequired &&
		order >= rejoinPhaseOrder(RejoinPhasePreflightPersisted) &&
		order <= rejoinPhaseOrder(RejoinPhaseSurvivorsAcknowledged) &&
		!transaction.IsolationConfirmed {
		return ErrRejoinJournalInvalid
	}
	if transaction.Phase != RejoinPhaseRecoveryRequired &&
		order >= rejoinPhaseOrder(RejoinPhaseReadinessVerified) &&
		transaction.IsolationConfirmed {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseTargetQuarantined) &&
		!transaction.TargetObserved {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseAttestationsPrepared) &&
		len(transaction.Attestations) != 3 {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseDeliveriesPrepared) &&
		(!fenceOperationPattern.MatchString(transaction.OperationID) ||
			transaction.Generation <= transaction.RemovalGeneration ||
			len(transaction.Deliveries) != 3 ||
			!fenceDigestPattern.MatchString(transaction.PrepareCheckpointDigest) ||
			!transaction.RepositoryPersisted) {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseDeliveriesInstalled) &&
		len(transaction.DeliveryReceipts) != 3 {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseActivationCommitted) &&
		!fenceDigestPattern.MatchString(transaction.ActivationCheckpointDigest) {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseSurvivorsAcknowledged) &&
		len(transaction.SurvivorReceipts) != 2 {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseReadinessVerified) &&
		!transaction.ReadinessVerified {
		return ErrRejoinJournalInvalid
	}
	if order >= rejoinPhaseOrder(RejoinPhaseCheckpointPersisted) &&
		(!fenceDigestPattern.MatchString(transaction.TargetReceipt) ||
			!fenceDigestPattern.MatchString(transaction.FinalCheckpointDigest)) {
		return ErrRejoinJournalInvalid
	}
	return nil
}

func sameRejoinBinding(
	transaction RejoinTransaction,
	request RejoinRequest,
	manifest topologyManifest,
	target nodeSpec,
) bool {
	expected := newRejoinTransaction(request, manifest, target)
	return SameRejoinTransactionBinding(transaction, expected)
}

// SameRejoinTransactionBinding reports whether two revisions describe the
// same immutable Rejoin request.
func SameRejoinTransactionBinding(left, right RejoinTransaction) bool {
	return left.Version == right.Version &&
		left.ManifestDigest == right.ManifestDigest &&
		left.TargetSlot == right.TargetSlot &&
		left.ChannelID == right.ChannelID &&
		left.ExpectedPrincipalHash == right.ExpectedPrincipalHash &&
		left.ExpectedWakuPeerIDHash == right.ExpectedWakuPeerIDHash &&
		left.ExpectedImageHash == right.ExpectedImageHash &&
		left.Actor == right.Actor && left.RequestID == right.RequestID &&
		left.StartedAt.Equal(right.StartedAt) &&
		left.Deadline.Equal(right.Deadline) &&
		left.FenceRequestID == right.FenceRequestID &&
		left.FenceEvidenceDigest == right.FenceEvidenceDigest &&
		left.RemovalOperationID == right.RemovalOperationID &&
		left.RemovalGeneration == right.RemovalGeneration &&
		left.RemovalCheckpointDigest == right.RemovalCheckpointDigest
}

// RejoinAuthorizationIntersection returns the exact existing Ardents
// action/resource owners needed by the R1 adapters. External service-manager,
// firewall, DNS/static-set, and peer-allow controls are deliberately absent.
func RejoinAuthorizationIntersection(
	target RejoinTarget,
	preparation RejoinPreparation,
) ([]RejoinAuthorizationBinding, error) {
	channelRaw, err := hex.DecodeString(target.ChannelID)
	if err != nil || len(channelRaw) != 16 ||
		!validRejoinPreparation(preparation, target) {
		return nil, ValidationError("topology_rejoin_authorization_invalid")
	}
	var channelID [16]byte
	copy(channelID[:], channelRaw)
	channelResource := authority.ChannelResource(target.RealmID, channelID)
	operationResource := authority.OperationResource(
		target.RealmID, preparation.OperationID,
	)
	bindings := []RejoinAuthorizationBinding{
		{Action: "realm.channel.membership.change", ResourceKind: "realm-channel", ResourceID: channelResource},
		{Action: "realm.channel.audit.read", ResourceKind: "realm", ResourceID: target.RealmID},
		{Action: "realm.channel.audit.read", ResourceKind: "realm-channel", ResourceID: channelResource},
		{Action: "node.start", ResourceKind: "node", ResourceID: target.TargetPrincipal},
		{Action: "node.runtime", ResourceKind: "node", ResourceID: target.TargetPrincipal},
		{Action: "node.features", ResourceKind: "node", ResourceID: target.TargetPrincipal},
		{Action: "transport.network_status", ResourceKind: "network", ResourceID: target.TargetSlot},
		{Action: "config.effective", ResourceKind: "configuration", ResourceID: target.TargetSlot},
		{Action: "config.reload", ResourceKind: "configuration", ResourceID: target.TargetSlot},
		{Action: "realm.channel.activation.commit", ResourceKind: "realm-channel-operation", ResourceID: operationResource},
	}
	for _, principal := range target.RecipientPrincipals {
		delivery := preparation.Deliveries[principal]
		deliveryResource, ok := authority.GenerationDeliveryResource(
			target.RealmID, preparation.OperationID, delivery.DeliveryID,
		)
		if !ok {
			return nil, ValidationError("topology_rejoin_authorization_invalid")
		}
		bindings = append(bindings,
			RejoinAuthorizationBinding{
				Action: "realm.channel.delivery.prepare", ResourceKind: "principal",
				ResourceID: principal, RecipientPrincipal: principal,
			},
			RejoinAuthorizationBinding{
				Action: "realm.channel.delivery.install", ResourceKind: "realm-channel-delivery",
				ResourceID: deliveryResource, RecipientPrincipal: principal,
			},
			RejoinAuthorizationBinding{
				Action: "realm.channel.delivery.acknowledge", ResourceKind: "realm-channel-delivery",
				ResourceID: deliveryResource, RecipientPrincipal: principal,
			},
			RejoinAuthorizationBinding{
				Action: "realm.channel.generation.activate", ResourceKind: "realm-channel-operation",
				ResourceID: operationResource, RecipientPrincipal: principal,
			},
			RejoinAuthorizationBinding{
				Action: "realm.channel.activation.acknowledge", ResourceKind: "realm-channel-delivery",
				ResourceID: deliveryResource, RecipientPrincipal: principal,
			},
		)
	}
	sort.Slice(bindings, func(left, right int) bool {
		if bindings[left].Action != bindings[right].Action {
			return bindings[left].Action < bindings[right].Action
		}
		if bindings[left].ResourceID != bindings[right].ResourceID {
			return bindings[left].ResourceID < bindings[right].ResourceID
		}
		return bindings[left].RecipientPrincipal < bindings[right].RecipientPrincipal
	})
	return bindings, nil
}

// ValidRejoinTransactionTransition accepts only the coordinator's monotonic
// next revision. Protected Authority and recipient evidence cannot be
// rewritten after first persistence.
func ValidRejoinTransactionTransition(
	previous RejoinTransaction,
	next RejoinTransaction,
) bool {
	if next.Revision != previous.Revision+1 ||
		!SameRejoinTransactionBinding(previous, next) ||
		!validRejoinPhaseTransition(previous, next) {
		return false
	}
	if previous.OperationID != "" && previous.OperationID != next.OperationID ||
		previous.Generation != 0 && previous.Generation != next.Generation ||
		previous.PrepareCheckpointDigest != "" &&
			previous.PrepareCheckpointDigest != next.PrepareCheckpointDigest ||
		previous.ActivationCheckpointDigest != "" &&
			previous.ActivationCheckpointDigest != next.ActivationCheckpointDigest ||
		previous.TargetReceipt != "" && previous.TargetReceipt != next.TargetReceipt ||
		previous.FinalCheckpointDigest != "" &&
			previous.FinalCheckpointDigest != next.FinalCheckpointDigest {
		return false
	}
	if previous.TargetObserved && !next.TargetObserved ||
		previous.RepositoryPersisted && !next.RepositoryPersisted {
		return false
	}
	if previous.Attestations != nil &&
		!reflect.DeepEqual(previous.Attestations, next.Attestations) ||
		previous.Deliveries != nil &&
			!reflect.DeepEqual(previous.Deliveries, next.Deliveries) ||
		previous.DeliveryReceipts != nil &&
			!reflect.DeepEqual(previous.DeliveryReceipts, next.DeliveryReceipts) ||
		previous.SurvivorReceipts != nil &&
			!reflect.DeepEqual(previous.SurvivorReceipts, next.SurvivorReceipts) {
		return false
	}
	return true
}

func validRejoinPhaseTransition(
	previous RejoinTransaction,
	next RejoinTransaction,
) bool {
	if next.Phase == RejoinPhaseRecoveryRequired {
		if previous.Phase == RejoinPhaseRecoveryRequired {
			return next.ResumeFrom == previous.ResumeFrom &&
				next.FailureReason != "" &&
				(!previous.IsolationConfirmed || next.IsolationConfirmed)
		}
		return next.ResumeFrom == previous.Phase && next.FailureReason != "" &&
			previous.Phase != RejoinPhaseRejoined &&
			previous.Phase != RejoinPhaseRecoveryRequired
	}
	if previous.Phase == RejoinPhaseRecoveryRequired {
		return next.Phase == previous.ResumeFrom &&
			next.ResumeFrom == "" && next.FailureReason == "" &&
			previous.IsolationConfirmed
	}
	if previous.Phase == RejoinPhaseRestorationPending &&
		next.Phase == RejoinPhaseRestorationPending {
		return !previous.RestorationApplied && next.RestorationApplied
	}
	return rejoinPhaseOrder(next.Phase) == rejoinPhaseOrder(previous.Phase)+1 &&
		next.ResumeFrom == "" && next.FailureReason == ""
}

func persistRejoin(
	ctx context.Context,
	store RejoinJournalStore,
	transaction *RejoinTransaction,
) error {
	expected := transaction.Revision
	transaction.Revision++
	if err := store.Save(ctx, expected, cloneRejoinTransaction(*transaction)); err != nil {
		transaction.Revision = expected
		return err
	}
	return nil
}

func rejoinStatus(transaction RejoinTransaction) RejoinStatus {
	outcome := RejoinOutcomeRecoveryRequired
	if transaction.Phase == RejoinPhaseRejoined {
		outcome = RejoinOutcomeRejoined
	}
	return RejoinStatus{
		APIVersion: RejoinStatusVersion, TargetSlot: transaction.TargetSlot,
		Outcome: outcome, Phase: transaction.Phase, Reason: transaction.FailureReason,
		DeliveryCount: len(transaction.Deliveries),
		SurvivorCount: len(transaction.SurvivorReceipts),
		Ready:         transaction.Phase == RejoinPhaseRejoined,
	}
}

func dependencyRejoinReason(
	err error,
	fallback RejoinFailureReason,
) RejoinFailureReason {
	var dependency RejoinDependencyError
	if errors.As(err, &dependency) {
		return RejoinFailureReason(dependency)
	}
	return fallback
}

func rejoinPhaseOrder(phase RejoinPhase) int {
	switch phase {
	case RejoinPhaseRequested:
		return 1
	case RejoinPhasePreflightPersisted:
		return 2
	case RejoinPhaseTargetQuarantined:
		return 3
	case RejoinPhaseAttestationsPrepared:
		return 4
	case RejoinPhaseAuthorityPending:
		return 5
	case RejoinPhaseDeliveriesPrepared:
		return 6
	case RejoinPhaseDeliveriesInstalled:
		return 7
	case RejoinPhaseActivationCommitted:
		return 8
	case RejoinPhaseSurvivorsAcknowledged:
		return 9
	case RejoinPhaseRestorationPending:
		return 10
	case RejoinPhaseReadinessVerified:
		return 11
	case RejoinPhaseTargetAcknowledgementPending:
		return 12
	case RejoinPhaseCheckpointPersisted:
		return 13
	case RejoinPhaseRejoined:
		return 14
	case RejoinPhaseRecoveryRequired:
		return 15
	default:
		return 0
	}
}

func resumableRejoinPhase(phase RejoinPhase) bool {
	order := rejoinPhaseOrder(phase)
	return order >= rejoinPhaseOrder(RejoinPhaseRequested) &&
		order < rejoinPhaseOrder(RejoinPhaseRejoined)
}

func cloneRejoinTransaction(transaction RejoinTransaction) RejoinTransaction {
	transaction.Attestations = cloneRejoinAttestations(transaction.Attestations)
	transaction.Deliveries = cloneRejoinDeliveries(transaction.Deliveries)
	transaction.DeliveryReceipts = cloneStringMap(transaction.DeliveryReceipts)
	transaction.SurvivorReceipts = cloneStringMap(transaction.SurvivorReceipts)
	return transaction
}

func cloneRejoinAttestations(
	values map[string]RejoinAttestation,
) map[string]RejoinAttestation {
	if values == nil {
		return nil
	}
	cloned := make(map[string]RejoinAttestation, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneRejoinDeliveries(
	values map[string]RejoinDelivery,
) map[string]RejoinDelivery {
	if values == nil {
		return nil
	}
	cloned := make(map[string]RejoinDelivery, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
