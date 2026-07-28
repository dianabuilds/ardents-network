package authority

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	identityapi "ardents/internal/identity"
)

const (
	MaxDeploymentFenceControls = 16
	MaxDeploymentFenceReason   = 128
	MaxFenceControlKindBytes   = 64
	MaxFencePrincipalBytes     = 128
	MaxDeploymentClockSkew     = 30
	MaxFenceEvidenceAge        = 5 * time.Minute
)

var fenceDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type MembershipChangeRequest struct {
	Version               uint32
	RequestID             string
	RealmID               string
	ChannelID             [16]byte
	Change                MembershipChangeKind
	TargetPrincipal       string
	RecipientAttestations []identityapi.CapabilityDeliveryAttestation
	ValidFor              time.Duration
	DrainFor              time.Duration
}

type FenceEvidenceRequest struct {
	Version     uint32
	RealmID     string
	ChannelID   [16]byte
	OperationID string
	Evidence    DeploymentFenceEvidence
}

type FenceEvidenceResult struct {
	Version           uint32
	RealmID           string
	OperationID       string
	AuthoritySequence uint64
	Phase             string
	TargetPrincipal   string
	EvidenceDigest    string
}

func (s *Service) ChangeChannelMembership(
	ctx context.Context,
	command Command,
	request MembershipChangeRequest,
) (RotationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotateChannelLocked(ctx, command, RotationRequest{
		Version: request.Version, RequestID: request.RequestID,
		RealmID: request.RealmID, ChannelID: request.ChannelID,
		RecipientAttestations: request.RecipientAttestations,
		ValidFor:              request.ValidFor, DrainFor: request.DrainFor,
		MembershipChange: request.Change, TargetPrincipal: request.TargetPrincipal,
	})
}

func (s *Service) SubmitDeploymentFenceEvidence(
	ctx context.Context,
	command Command,
	request FenceEvidenceRequest,
) (FenceEvidenceResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Version != ContractVersion {
		return FenceEvidenceResult{}, ErrUnsupportedVersion
	}
	if !ValidRealmID(request.RealmID) || zeroFixedID(request.ChannelID) ||
		!operationIDPattern.MatchString(request.OperationID) ||
		command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionChangeMembership ||
		command.ResourceKind != ResourceKindChannel ||
		command.ResourceID != ChannelResource(request.RealmID, request.ChannelID) {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return FenceEvidenceResult{}, ErrUnavailable
	}
	if err := s.policy.AdmitChannelMembership(ctx, command); err != nil {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return FenceEvidenceResult{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		return FenceEvidenceResult{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	channelIndex := channelRecordIndex(state, request.ChannelID)
	if channelIndex < 0 {
		return FenceEvidenceResult{}, ErrInvalidArgument
	}
	if err := s.policy.AdmitChannelClass(
		ctx, command, identityapi.CapabilityScope(state.Channels[channelIndex].Class),
	); err != nil {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return FenceEvidenceResult{}, err
		}
	}
	rotationIndex := rotationRecordIndex(state, request.OperationID)
	if rotationIndex < 0 {
		return FenceEvidenceResult{}, ErrInvalidArgument
	}
	rotation := state.Rotations[rotationIndex]
	if rotation.ChannelID != request.ChannelID || rotation.MembershipChange.Version == 0 {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	if rotation.Phase != DeliveryPhaseActivationCommitted &&
		rotation.Phase != DeliveryPhaseCompleted {
		return FenceEvidenceResult{}, ErrConflict
	}
	for _, retained := range rotation.FenceEvidence {
		if retained.TargetPrincipal != request.Evidence.TargetPrincipal {
			continue
		}
		if !equalFenceEvidence(retained, request.Evidence) {
			return FenceEvidenceResult{}, ErrConflict
		}
		return fenceEvidenceResult(state, rotation, retained), nil
	}
	now := s.clock().UTC().Truncate(time.Second)
	if !now.Before(rotation.Deadline) || !now.Before(rotation.DrainDeadline) {
		return FenceEvidenceResult{}, ErrInvalidArgument
	}
	if err := validateDeploymentFenceEvidence(
		request.Evidence, state.RealmID, rotation.OperationID, command.Actor, now,
	); err != nil {
		return FenceEvidenceResult{}, err
	}
	if !fenceTargetAllowed(state, rotation, request.Evidence.TargetPrincipal) {
		return FenceEvidenceResult{}, ErrPermissionDenied
	}
	if rotation.Phase == DeliveryPhaseCompleted {
		return FenceEvidenceResult{}, ErrConflict
	}
	if len(rotation.FenceEvidence) >= MaxMembersPerChannel+1 {
		return FenceEvidenceResult{}, ErrResourceExhausted
	}
	audit := newDeliveryAudit(
		rotationAuditID(
			command.Action,
			rotation.OperationID+"\x00"+request.Evidence.TargetPrincipal,
		),
		command, rotation.OperationID, state.AuditHead, now,
	)
	audit.TargetPrincipal = request.Evidence.TargetPrincipal
	audit.ChannelClass = rotation.ChannelClass
	audit.Generation = rotation.PendingGeneration
	audit.Hash = auditHash(audit)
	err = s.commitCheckpointTransition(ctx, &state, audit, now, func(next *Ledger, checkpoint SignedCheckpoint) error {
		record := &next.Rotations[rotationIndex]
		record.FenceEvidence = append(record.FenceEvidence, cloneFenceEvidence(request.Evidence))
		if membershipCompletionSatisfied(*next, *record) {
			completeMembershipRotation(next, record, checkpoint.AuthoritySequence)
		}
		return nil
	})
	if err != nil {
		return FenceEvidenceResult{}, err
	}
	return fenceEvidenceResult(state, state.Rotations[rotationIndex], request.Evidence), nil
}

func equalFenceEvidence(left, right DeploymentFenceEvidence) bool {
	if left.Version != right.Version ||
		left.RealmID != right.RealmID ||
		left.OperationID != right.OperationID ||
		left.TargetPrincipal != right.TargetPrincipal ||
		left.ManifestDigest != right.ManifestDigest ||
		left.RequestID != right.RequestID ||
		left.Reason != right.Reason ||
		!left.ObservedAt.Equal(right.ObservedAt) ||
		left.ClockSkewSecond != right.ClockSkewSecond ||
		len(left.Controls) != len(right.Controls) {
		return false
	}
	for index := range left.Controls {
		if left.Controls[index] != right.Controls[index] {
			return false
		}
	}
	return true
}

func validMembershipChangeInput(change MembershipChangeKind, target string) bool {
	switch change {
	case "":
		return target == ""
	case MembershipChangeAdd, MembershipChangeRemove:
		return strings.TrimSpace(target) == target && target != ""
	default:
		return false
	}
}

func membershipCapacity(state Ledger, recipientCount int) error {
	requiredAuditRecords := 2*recipientCount + 3
	if recipientCount <= 0 || recipientCount > MaxMembersPerChannel ||
		len(state.Operations) >= MaxOperations || len(state.Rotations) >= MaxOperations ||
		len(state.AuditLog)+requiredAuditRecords > MaxAuditRecords ||
		len(state.AuditOutbox)+requiredAuditRecords > MaxAuditOutboxRecords {
		return ErrResourceExhausted
	}
	return nil
}

func validMembershipRecipients(
	change MembershipChangeKind,
	target string,
	current map[string]identityapi.CapabilityGrant,
	recipients map[string]struct{},
) bool {
	switch change {
	case "":
		if len(recipients) != len(current) {
			return false
		}
		for principal := range current {
			if _, ok := recipients[principal]; !ok {
				return false
			}
		}
		return true
	case MembershipChangeAdd:
		if _, exists := current[target]; exists || len(current) >= MaxMembersPerChannel ||
			len(recipients) != len(current)+1 {
			return false
		}
		if _, ok := recipients[target]; !ok {
			return false
		}
		for principal := range current {
			if _, ok := recipients[principal]; !ok {
				return false
			}
		}
		return true
	case MembershipChangeRemove:
		if _, exists := current[target]; !exists || len(current) <= 1 ||
			len(recipients) != len(current)-1 {
			return false
		}
		if _, present := recipients[target]; present {
			return false
		}
		for principal := range current {
			if principal == target {
				continue
			}
			if _, ok := recipients[principal]; !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func membershipChangeRecord(state Ledger, request RotationRequest) MembershipChangeRecord {
	if request.MembershipChange == "" {
		return MembershipChangeRecord{}
	}
	record := MembershipChangeRecord{
		Version: ContractVersion, Kind: request.MembershipChange,
		TargetPrincipal:   request.TargetPrincipal,
		MembershipVersion: state.AuthoritySequence,
		State:             MemberStateCandidate,
	}
	if request.MembershipChange == MembershipChangeAdd {
		record.PriorState = MemberStateRemoved
		record.PendingState = MemberStateCandidate
		record.TerminalState = MemberStateActive
	} else {
		record.PriorState = MemberStateActive
		record.PendingState = MemberStateSuspended
		record.TerminalState = MemberStateRemoved
		record.State = MemberStateSuspended
	}
	return record
}

func operationKind(request RotationRequest) string {
	if request.MembershipChange != "" {
		return "channel_membership"
	}
	if request.Renewal {
		return "channel_renewal"
	}
	return "channel_rotation"
}

func applyMembershipTruth(
	state *Ledger,
	channelID [16]byte,
	change MembershipChangeRecord,
) {
	if change.Version == 0 {
		return
	}
	index := -1
	for candidate := range state.Members {
		if state.Members[candidate].Principal == change.TargetPrincipal {
			index = candidate
			break
		}
	}
	switch change.Kind {
	case MembershipChangeAdd:
		if index < 0 {
			state.Members = append(state.Members, MemberRecord{
				Version: ContractVersion, Principal: change.TargetPrincipal,
			})
			sort.Slice(state.Members, func(i, j int) bool {
				return state.Members[i].Principal < state.Members[j].Principal
			})
		}
	case MembershipChangeRemove:
		retainedBySiblingChannel := false
		for _, channel := range state.Channels {
			if channel.ID == channelID {
				continue
			}
			for _, grant := range channelCurrentGrants(channel) {
				if grant.SubjectPrincipal == change.TargetPrincipal {
					retainedBySiblingChannel = true
					break
				}
			}
			if retainedBySiblingChannel {
				break
			}
		}
		if index >= 0 && !retainedBySiblingChannel {
			state.Members = append(state.Members[:index], state.Members[index+1:]...)
		}
	}
}

func validateDeploymentFenceEvidence(
	evidence DeploymentFenceEvidence,
	realmID, operationID, actor string,
	now time.Time,
) error {
	if evidence.Version != ContractVersion || evidence.RealmID != realmID ||
		evidence.OperationID != operationID ||
		strings.TrimSpace(evidence.TargetPrincipal) != evidence.TargetPrincipal ||
		evidence.TargetPrincipal == "" ||
		len(evidence.TargetPrincipal) > MaxFencePrincipalBytes ||
		!fenceDigestPattern.MatchString(evidence.ManifestDigest) ||
		len(evidence.RequestID) == 0 || len(evidence.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(evidence.RequestID) != evidence.RequestID ||
		len(evidence.Reason) == 0 || len(evidence.Reason) > MaxDeploymentFenceReason ||
		strings.TrimSpace(evidence.Reason) != evidence.Reason ||
		!canonicalSecond(evidence.ObservedAt) ||
		evidence.ObservedAt.After(now) || now.Sub(evidence.ObservedAt) > MaxFenceEvidenceAge ||
		evidence.ClockSkewSecond < -MaxDeploymentClockSkew ||
		evidence.ClockSkewSecond > MaxDeploymentClockSkew ||
		len(evidence.Controls) == 0 || len(evidence.Controls) > MaxDeploymentFenceControls {
		return ErrInvalidArgument
	}
	required := map[string]bool{
		"target_ingress_blocked": false,
		"discovery_withdrawn":    false,
		"peer_id_denied":         false,
	}
	seen := make(map[string]struct{}, len(evidence.Controls))
	for _, control := range evidence.Controls {
		if control.Actor != actor || len(control.Actor) > MaxFencePrincipalBytes ||
			strings.TrimSpace(control.Kind) != control.Kind || control.Kind == "" ||
			len(control.Kind) > MaxFenceControlKindBytes ||
			!fenceDigestPattern.MatchString(control.ReceiptDigest) {
			return ErrPermissionDenied
		}
		if _, duplicate := seen[control.Kind]; duplicate {
			return ErrInvalidArgument
		}
		seen[control.Kind] = struct{}{}
		if _, ok := required[control.Kind]; ok {
			required[control.Kind] = true
		}
	}
	for _, present := range required {
		if !present {
			return ErrInvalidArgument
		}
	}
	return nil
}

func validateStoredFenceEvidence(evidence DeploymentFenceEvidence) error {
	if len(evidence.Controls) == 0 {
		return ErrCorruptState
	}
	return validateDeploymentFenceEvidence(
		evidence, evidence.RealmID, evidence.OperationID,
		evidence.Controls[0].Actor, evidence.ObservedAt,
	)
}

func fenceTargetAllowed(state Ledger, rotation RotationRecord, target string) bool {
	if rotation.MembershipChange.Kind == MembershipChangeAdd &&
		target == rotation.MembershipChange.TargetPrincipal {
		return false
	}
	if rotation.MembershipChange.Kind == MembershipChangeRemove &&
		target == rotation.MembershipChange.TargetPrincipal {
		return true
	}
	for _, deliveryID := range rotation.DeliveryIDs {
		index := deliveryRecordIndex(state, deliveryID)
		if index >= 0 && state.InitialGenerationDeliveries[index].RecipientPrincipal == target {
			return true
		}
	}
	return false
}

func membershipCompletionSatisfied(state Ledger, rotation RotationRecord) bool {
	if rotation.MembershipChange.Version == 0 {
		return rotationActiveReceiptsComplete(state, rotation)
	}
	if rotation.MembershipChange.Kind == MembershipChangeRemove &&
		!rotationHasFence(rotation, rotation.MembershipChange.TargetPrincipal) {
		return false
	}
	for _, deliveryID := range rotation.DeliveryIDs {
		index := deliveryRecordIndex(state, deliveryID)
		if index < 0 {
			return false
		}
		delivery := state.InitialGenerationDeliveries[index]
		if delivery.ActiveReceipt.Phase != "active" &&
			!rotationHasFence(rotation, delivery.RecipientPrincipal) {
			return false
		}
	}
	return len(rotation.DeliveryIDs) > 0
}

func rotationHasFence(rotation RotationRecord, principal string) bool {
	for _, evidence := range rotation.FenceEvidence {
		if evidence.TargetPrincipal == principal {
			return true
		}
	}
	return false
}

func completeMembershipRotation(state *Ledger, rotation *RotationRecord, sequence uint64) {
	rotation.Phase = DeliveryPhaseCompleted
	rotation.CompletionSequence = sequence
	if rotation.MembershipChange.Version != 0 {
		rotation.MembershipChange.State = rotation.MembershipChange.TerminalState
	}
	setOperationPhase(state, rotation.OperationID, DeliveryPhaseCompleted)
}

func deploymentFenceEvidenceDigest(evidence DeploymentFenceEvidence) string {
	copy := cloneFenceEvidence(evidence)
	sort.Slice(copy.Controls, func(i, j int) bool {
		return copy.Controls[i].Kind < copy.Controls[j].Kind
	})
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(append([]byte("ardents:deployment-fence-evidence:v1\x00"), raw...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneFenceEvidence(evidence DeploymentFenceEvidence) DeploymentFenceEvidence {
	evidence.Controls = append([]DeploymentFenceControl(nil), evidence.Controls...)
	return evidence
}

func fenceEvidenceResult(
	state Ledger,
	rotation RotationRecord,
	evidence DeploymentFenceEvidence,
) FenceEvidenceResult {
	sequence := state.AuthoritySequence
	if rotation.CompletionSequence != 0 {
		sequence = rotation.CompletionSequence
	}
	return FenceEvidenceResult{
		Version: ContractVersion, RealmID: state.RealmID,
		OperationID: rotation.OperationID, AuthoritySequence: sequence,
		Phase: rotation.Phase, TargetPrincipal: evidence.TargetPrincipal,
		EvidenceDigest: deploymentFenceEvidenceDigest(evidence),
	}
}
