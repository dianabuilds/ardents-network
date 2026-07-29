package authority

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
)

func validateLedger(state Ledger) error {
	if state.Version != ContractVersion || state.SchemaVersion != SchemaVersion ||
		state.Revision == 0 || !ValidRealmID(state.RealmID) ||
		state.RealmClass != RealmClassProduction || state.AuthorityEpoch == 0 ||
		state.AuthoritySequence == 0 || state.AuthorityPrincipal == "" ||
		len(state.AuthorityPublicKey) != 32 || !digestPattern.MatchString(state.AuditHead) {
		return corruptLedger("header")
	}
	if state.Phase != PhaseCheckpointing && state.Phase != PhaseReady && state.Phase != PhaseRecoveryRequired {
		return corruptLedger("phase")
	}
	switch state.Phase {
	case PhaseCheckpointing:
		if state.Readiness != ReadinessDegraded || state.Reason != ReasonCheckpointMissing {
			return corruptLedger("checkpointing readiness")
		}
	case PhaseReady:
		if state.Readiness != ReadinessReady || state.Reason != ReasonNone {
			return corruptLedger("ready readiness")
		}
	case PhaseRecoveryRequired:
		if state.Readiness != ReadinessRecoveryRequired || state.Reason == ReasonNone {
			return corruptLedger("recovery readiness")
		}
	}
	if state.Checkpoint.AuthorityPrincipal != state.AuthorityPrincipal ||
		!equalBytes(state.Checkpoint.AuthorityPublicKey, state.AuthorityPublicKey) ||
		state.Checkpoint.AuditHead != state.AuditHead {
		return corruptLedger("checkpoint binding")
	}
	if len(state.Members) > MaxRealmMembers || len(state.Channels) > MaxActiveChannels ||
		len(state.Operations) == 0 || len(state.Operations) > MaxOperations ||
		len(state.Idempotency) == 0 || len(state.Idempotency) > MaxIdempotencyRecords ||
		len(state.AuditLog) == 0 || len(state.AuditLog) > MaxAuditRecords ||
		len(state.AuditOutbox) > MaxAuditOutboxRecords {
		return corruptLedger("container bounds")
	}
	memberPrincipals := make(map[string]struct{}, len(state.Members))
	for _, member := range state.Members {
		if member.Version != ContractVersion || member.Principal == "" {
			return corruptLedger("member")
		}
		if _, duplicate := memberPrincipals[member.Principal]; duplicate {
			return corruptLedger("duplicate member")
		}
		memberPrincipals[member.Principal] = struct{}{}
	}
	channelIDs := make(map[[16]byte]struct{}, len(state.Channels))
	for _, channel := range state.Channels {
		if zeroFixedID(channel.ID) ||
			!validChannelClass(identityapi.CapabilityScope(channel.Class)) {
			return corruptLedger("channel identity")
		}
		if _, duplicate := channelIDs[channel.ID]; duplicate {
			return corruptLedger("duplicate channel")
		}
		channelIDs[channel.ID] = struct{}{}
		if channel.MemberCount > MaxMembersPerChannel ||
			channel.PendingGenerationCount > MaxPendingGenerations ||
			channel.PreviousReceiveGenerationCount > MaxPreviousReceiveGenerations ||
			channel.OutstandingDeliveryCount > MaxOutstandingDeliveries*channel.MemberCount {
			return corruptLedger("channel bounds")
		}
		current := channelCurrentGrants(channel)
		if channel.MemberCount == 0 || len(current) != int(channel.MemberCount) ||
			validateChannelGrantSetForLedger(
				state, channel, current, channel.CurrentGeneration,
			) != nil {
			return corruptLedger("channel grant")
		}
		for _, stored := range current {
			if _, ok := memberPrincipals[stored.SubjectPrincipal]; !ok {
				return corruptLedger("channel member")
			}
		}
		if channel.PendingGenerationCount == 0 {
			if len(channel.PendingGrants) != 0 {
				return corruptLedger("pending channel grant")
			}
		} else if len(channel.PendingGrants) == 0 ||
			len(channel.PendingGrants) > MaxMembersPerChannel ||
			validateChannelGrantSetForLedger(
				state, channel, channel.PendingGrants, channel.CurrentGeneration+1,
			) != nil {
			return corruptLedger("pending channel grant")
		}
		if channel.PreviousReceiveGenerationCount == 0 {
			if len(channel.PreviousGrants) != 0 || !channel.PreviousDrainDeadline.IsZero() {
				return corruptLedger("previous channel grant")
			}
		} else if channel.CurrentGeneration < 2 ||
			len(channel.PreviousGrants) == 0 ||
			len(channel.PreviousGrants) > MaxMembersPerChannel ||
			!canonicalSecond(channel.PreviousDrainDeadline) ||
			validateChannelGrantSetForLedger(
				state, channel, channel.PreviousGrants, channel.CurrentGeneration-1,
			) != nil {
			return corruptLedger("previous channel grant")
		}
	}
	for _, operation := range state.Operations {
		if operation.Version != ContractVersion || !operationIDPattern.MatchString(operation.ID) ||
			len(operation.RequestID) == 0 || len(operation.RequestID) > MaxRequestIDBytes ||
			!canonicalSecond(operation.CreatedAt) ||
			!canonicalSecond(operation.Deadline) || !operation.Deadline.After(operation.CreatedAt) ||
			operation.Deadline.Sub(operation.CreatedAt) > MaxOperationLifetime {
			return corruptLedger("genesis operation")
		}
		switch operation.Kind {
		case "realm_genesis":
			if operation.Phase != PhaseCheckpointing && operation.Phase != PhaseReady &&
				operation.Phase != PhaseRecoveryRequired {
				return corruptLedger("genesis operation")
			}
		case "channel_rotation", "channel_membership", "channel_renewal":
			if operation.Phase != DeliveryPhaseDelivering &&
				operation.Phase != DeliveryPhaseInstalled &&
				operation.Phase != DeliveryPhaseActivationCommitted &&
				operation.Phase != DeliveryPhaseCompleted &&
				operation.Phase != PhaseRecoveryRequired {
				return corruptLedger("rotation operation")
			}
		case "authority_transition":
			if operation.Phase != PhaseCheckpointing &&
				operation.Phase != PhaseReady &&
				operation.Phase != PhaseRecoveryRequired {
				return corruptLedger("authority transition operation")
			}
		default:
			return corruptLedger("operation kind")
		}
	}
	for _, record := range state.Idempotency {
		genesisDigest := state.GenesisCheckpointDigest
		if genesisDigest == "" && state.AuthoritySequence == 1 {
			genesisDigest = state.Checkpoint.Digest
		}
		if record.Version != ContractVersion || len(record.RequestID) == 0 ||
			len(record.RequestID) > MaxRequestIDBytes || len(record.PayloadHash) != sha256.Size*2 ||
			record.Result.Version != ContractVersion || record.Result.RealmID != state.RealmID ||
			record.Result.AuthorityEpoch != 1 ||
			record.Result.AuthoritySequence != 1 ||
			record.Result.CheckpointDigest != genesisDigest ||
			!operationIDPattern.MatchString(record.Result.OperationID) {
			return corruptLedger("genesis idempotency")
		}
		if _, err := hex.DecodeString(record.PayloadHash); err != nil {
			return corruptLedger("genesis payload hash")
		}
	}
	for _, delivery := range state.InitialGenerationDeliveries {
		if err := validateInitialGenerationDeliveryRecord(state, delivery); err != nil {
			return corruptLedger("initial generation delivery")
		}
	}
	for _, rotation := range state.Rotations {
		if err := validateRotationRecord(state, rotation); err != nil {
			return corruptLedger("rotation")
		}
	}
	if err := validateMigrationRecord(state); err != nil {
		return corruptLedger("migration")
	}
	if err := validateAuthorityTransitionRecord(state); err != nil {
		return corruptLedger("authority transition")
	}
	if state.AuthoritySequence == 1 && len(state.InitialGenerationDeliveries) != 0 {
		return corruptLedger("sequence one deliveries")
	}
	previousAuditHash := ""
	auditByID := make(map[string]AuditRecord, len(state.AuditLog))
	for _, record := range state.AuditLog {
		if err := validateAuditRecord(record, previousAuditHash); err != nil {
			return corruptLedger("audit chain")
		}
		if _, duplicate := auditByID[record.ID]; duplicate {
			return corruptLedger("duplicate audit")
		}
		auditByID[record.ID] = record
		previousAuditHash = record.Hash
	}
	if previousAuditHash != state.AuditHead {
		return corruptLedger("audit head")
	}
	for _, record := range state.AuditOutbox {
		retained, ok := auditByID[record.ID]
		if !ok || retained != record {
			return corruptLedger("audit outbox")
		}
	}
	if err := validateFenceEvidenceAuditBindings(state, auditByID); err != nil {
		return corruptLedger("fence evidence audit binding")
	}
	if err := ValidateCheckpoint(state.Checkpoint); err != nil ||
		state.Checkpoint.RealmID != state.RealmID ||
		state.Checkpoint.AuthoritySequence != state.AuthoritySequence ||
		state.Checkpoint.Digest == "" {
		return corruptLedger("checkpoint")
	}
	return nil
}

func validateFenceEvidenceAuditBindings(
	state Ledger,
	auditByID map[string]AuditRecord,
) error {
	bound := make(map[string]struct{})
	for _, rotation := range state.Rotations {
		channelIndex := channelRecordIndex(state, rotation.ChannelID)
		if channelIndex < 0 {
			return ErrCorruptState
		}
		for _, evidence := range rotation.FenceEvidence {
			auditID := rotationAuditID(
				ActionFenceNode,
				rotation.OperationID+"\x00"+evidence.TargetPrincipal,
			)
			audit, ok := auditByID[auditID]
			if !ok ||
				audit.Action != ActionFenceNode ||
				audit.OperationID != rotation.OperationID ||
				audit.TargetPrincipal != evidence.TargetPrincipal ||
				audit.Actor != evidence.Controls[0].Actor ||
				audit.ResourceKind != ResourceKindNode ||
				audit.ResourceID != FenceNodeResource(evidence.TargetPrincipal) ||
				audit.ChannelClass != state.Channels[channelIndex].Class ||
				audit.Generation != rotation.PendingGeneration ||
				audit.EvidenceDigest != DeploymentFenceEvidenceDigest(evidence) {
				return ErrCorruptState
			}
			bound[auditID] = struct{}{}
		}
	}
	for auditID, audit := range auditByID {
		if audit.Action != ActionFenceNode {
			continue
		}
		if _, ok := bound[auditID]; !ok {
			return ErrCorruptState
		}
	}
	return nil
}

func corruptLedger(section string) error {
	return fmt.Errorf("%w: %s", ErrCorruptState, section)
}

func cloneLedger(state Ledger) Ledger {
	state.AuthorityPublicKey = append([]byte(nil), state.AuthorityPublicKey...)
	state.Checkpoint.AuthorityPublicKey = append([]byte(nil), state.Checkpoint.AuthorityPublicKey...)
	state.Checkpoint.Signature = append([]byte(nil), state.Checkpoint.Signature...)
	if state.Checkpoint.AuthorityTransition != nil {
		transition := cloneAuthorityTransitionValue(*state.Checkpoint.AuthorityTransition)
		state.Checkpoint.AuthorityTransition = &transition
	}
	state.Members = append([]MemberRecord(nil), state.Members...)
	state.Channels = append([]ChannelRecord(nil), state.Channels...)
	for index := range state.Channels {
		state.Channels[index].Grant.Secret = append([]byte(nil), state.Channels[index].Grant.Secret...)
		state.Channels[index].Grant.Signature = append([]byte(nil), state.Channels[index].Grant.Signature...)
		state.Channels[index].CurrentGrants = cloneGrantRecords(state.Channels[index].CurrentGrants)
		state.Channels[index].PendingGrants = cloneGrantRecords(state.Channels[index].PendingGrants)
		state.Channels[index].PreviousGrants = cloneGrantRecords(state.Channels[index].PreviousGrants)
	}
	state.Operations = append([]OperationRecord(nil), state.Operations...)
	state.Idempotency = append([]IdempotencyRecord(nil), state.Idempotency...)
	state.AuditLog = append([]AuditRecord(nil), state.AuditLog...)
	state.AuditOutbox = append([]AuditRecord(nil), state.AuditOutbox...)
	state.InitialGenerationDeliveries = append(
		[]InitialGenerationDeliveryRecord(nil), state.InitialGenerationDeliveries...,
	)
	for index := range state.InitialGenerationDeliveries {
		record := &state.InitialGenerationDeliveries[index]
		record.ReceiptKey = append([]byte(nil), record.ReceiptKey...)
		record.Sealed.Envelope = append([]byte(nil), record.Sealed.Envelope...)
		record.Receipt.MAC = append([]byte(nil), record.Receipt.MAC...)
		record.ActiveReceipt.MAC = append([]byte(nil), record.ActiveReceipt.MAC...)
	}
	state.Rotations = append([]RotationRecord(nil), state.Rotations...)
	for index := range state.Rotations {
		state.Rotations[index].DeliveryIDs = append([]string(nil), state.Rotations[index].DeliveryIDs...)
		state.Rotations[index].FenceEvidence = append(
			[]DeploymentFenceEvidence(nil), state.Rotations[index].FenceEvidence...,
		)
		for evidenceIndex := range state.Rotations[index].FenceEvidence {
			state.Rotations[index].FenceEvidence[evidenceIndex] = cloneFenceEvidence(
				state.Rotations[index].FenceEvidence[evidenceIndex],
			)
		}
		state.Rotations[index].Activation.Signature = append(
			[]byte(nil), state.Rotations[index].Activation.Signature...,
		)
	}
	if state.Migration != nil {
		migration := *state.Migration
		migration.RequiredRotationChannelIDs = append(
			[][16]byte(nil), migration.RequiredRotationChannelIDs...,
		)
		migration.RotatedChannelIDs = append(
			[][16]byte(nil), migration.RotatedChannelIDs...,
		)
		state.Migration = &migration
	}
	if state.Transition != nil {
		transition := *state.Transition
		transition.Proof = cloneAuthorityTransitionValue(transition.Proof)
		transition.RequiredRotationChannelIDs = append(
			[][16]byte(nil), transition.RequiredRotationChannelIDs...,
		)
		transition.RotatedChannelIDs = append(
			[][16]byte(nil), transition.RotatedChannelIDs...,
		)
		if transition.Completion != nil {
			completion := *transition.Completion
			completion.RequiredRotationChannelIDs = append(
				[][16]byte(nil), completion.RequiredRotationChannelIDs...,
			)
			completion.RotatedChannelIDs = append(
				[][16]byte(nil), completion.RotatedChannelIDs...,
			)
			completion.Signature = append([]byte(nil), completion.Signature...)
			transition.Completion = &completion
		}
		state.Transition = &transition
	}
	return state
}

func canonicalSecond(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC &&
		value.Nanosecond() == 0
}

func validateAuditRecord(record AuditRecord, previousHash string) error {
	if record.Version != ContractVersion || !auditIDPattern.MatchString(record.ID) ||
		record.Actor == "" || record.Actor != record.Effective ||
		!operationIDPattern.MatchString(record.OperationID) || record.Outcome != "accepted" ||
		record.PreviousHash != previousHash || !canonicalSecond(record.CreatedAt) ||
		record.Hash != auditHash(record) {
		return ErrCorruptState
	}
	targetPrincipalAudit := record.Action == ActionChangeMembership ||
		record.Action == ActionFenceNode
	if targetPrincipalAudit != (record.TargetPrincipal != "") {
		return ErrCorruptState
	}
	switch record.Action {
	case ActionMigrateLocalV2:
		if !sha256Pattern.MatchString(record.EvidenceDigest) {
			return ErrCorruptState
		}
	case ActionFenceNode:
		if !fenceDigestPattern.MatchString(record.EvidenceDigest) {
			return ErrCorruptState
		}
	default:
		if record.EvidenceDigest != "" {
			return ErrCorruptState
		}
	}
	switch record.Action {
	case ActionCreate, ActionMigrateLocalV2:
		if record.ResourceKind != ResourceKindAuthorityInstance ||
			record.ResourceID != PrimaryAuthorityInstance {
			return ErrCorruptState
		}
	case ActionIssueDelivery, ActionAcknowledgeDelivery, ActionAcknowledgeActivation:
		if record.ResourceKind != ResourceKindGenerationDelivery ||
			!validGenerationDeliveryResource(record.ResourceID) {
			return ErrCorruptState
		}
	case ActionRotateGeneration, ActionChangeMembership:
		if record.ResourceKind != ResourceKindChannel ||
			!validChannelResource(record.ResourceID) {
			return ErrCorruptState
		}
	case ActionFenceNode:
		if record.ResourceKind != ResourceKindNode ||
			record.ResourceID != FenceNodeResource(record.TargetPrincipal) {
			return ErrCorruptState
		}
	case ActionCommitActivation:
		if record.ResourceKind != ResourceKindOperation ||
			!validOperationResource(record.ResourceID) {
			return ErrCorruptState
		}
	case ActionPlanTransition:
		if record.ResourceKind != ResourceKindRealm ||
			!ValidRealmID(record.ResourceID) {
			return ErrCorruptState
		}
	default:
		return ErrCorruptState
	}
	return nil
}

func validateInitialGenerationDeliveryRecord(state Ledger, record InitialGenerationDeliveryRecord) error {
	if record.Version != ContractVersion || len(record.RequestID) == 0 ||
		len(record.RequestID) > MaxRequestIDBytes || len(record.PayloadHash) != sha256.Size*2 ||
		!operationIDPattern.MatchString(record.OperationID) ||
		!deliveryIDPattern.MatchString(record.DeliveryID) ||
		record.RecipientPrincipal == "" || record.RetryGeneration != 0 ||
		len(record.ReceiptKey) != sha256.Size ||
		(record.Phase != DeliveryPhaseIssued && record.Phase != DeliveryPhaseInstalled) ||
		!canonicalSecond(record.CreatedAt) || !canonicalSecond(record.Deadline) ||
		!record.Deadline.After(record.CreatedAt) ||
		record.Deadline.Sub(record.CreatedAt) > MaxOperationLifetime ||
		record.Sealed.Binding.RealmID != state.RealmID ||
		record.Sealed.Binding.DeliveryID != record.DeliveryID ||
		record.Sealed.Binding.ChannelID != record.ChannelID ||
		record.Sealed.Binding.RecipientPrincipal != record.RecipientPrincipal {
		return ErrCorruptState
	}
	if _, err := hex.DecodeString(record.PayloadHash); err != nil {
		return ErrCorruptState
	}
	if record.Phase == DeliveryPhaseInstalled {
		if identitycapability.VerifyGenerationDeliveryReceipt(record.Receipt, record.ReceiptKey) != nil {
			return ErrCorruptState
		}
	} else if len(record.Receipt.MAC) != 0 {
		return ErrCorruptState
	}
	if len(record.ActiveReceipt.MAC) != 0 {
		if record.ActiveReceipt.Phase != identitycapability.DeliveryPhaseActive ||
			identitycapability.VerifyGenerationDeliveryReceipt(
				record.ActiveReceipt, record.ReceiptKey,
			) != nil {
			return ErrCorruptState
		}
	}
	return nil
}

func validateChannelGrantSet(
	channel ChannelRecord,
	records []CapabilityGrantRecord,
	generation uint32,
	authorityPublic ed25519.PublicKey,
) error {
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		grant, ok := record.restore()
		if !ok || channel.ID != grant.ChannelID ||
			channel.Class != string(grant.Scope) ||
			generation != grant.Generation ||
			identitycapability.VerifyGrant(grant, authorityPublic) != nil {
			return ErrCorruptState
		}
		if _, duplicate := seen[grant.SubjectPrincipal]; duplicate {
			return ErrCorruptState
		}
		seen[grant.SubjectPrincipal] = struct{}{}
	}
	return nil
}

func validateChannelGrantSetForLedger(
	state Ledger,
	channel ChannelRecord,
	records []CapabilityGrantRecord,
	generation uint32,
) error {
	if len(records) == 0 {
		return ErrCorruptState
	}
	public, ok := authorityPublicForPrincipal(state, records[0].IssuerPrincipal)
	if !ok {
		return ErrCorruptState
	}
	for _, record := range records {
		if record.IssuerPrincipal != records[0].IssuerPrincipal {
			return ErrCorruptState
		}
	}
	return validateChannelGrantSet(channel, records, generation, public)
}

func authorityPublicForPrincipal(state Ledger, principal string) (ed25519.PublicKey, bool) {
	if principal == state.AuthorityPrincipal {
		return ed25519.PublicKey(state.AuthorityPublicKey), true
	}
	if state.Transition != nil &&
		principal == state.Transition.Proof.FromAuthorityPrincipal {
		return ed25519.PublicKey(state.Transition.Proof.FromAuthorityPublicKey), true
	}
	return nil, false
}

func validateRotationRecord(state Ledger, rotation RotationRecord) error {
	if rotation.Version != ContractVersion ||
		len(rotation.RequestID) == 0 || len(rotation.RequestID) > MaxRequestIDBytes ||
		len(rotation.PayloadHash) != sha256.Size*2 ||
		!operationIDPattern.MatchString(rotation.OperationID) ||
		zeroFixedID(rotation.ChannelID) ||
		rotation.PreviousGeneration == 0 ||
		rotation.PendingGeneration != rotation.PreviousGeneration+1 ||
		rotation.PrepareSequence == 0 ||
		rotation.PrepareSequence > state.AuthoritySequence ||
		len(rotation.DeliveryIDs) == 0 ||
		len(rotation.DeliveryIDs) > MaxMembersPerChannel ||
		!canonicalSecond(rotation.CreatedAt) || !canonicalSecond(rotation.Deadline) ||
		!canonicalSecond(rotation.DrainDeadline) ||
		!rotation.Deadline.After(rotation.CreatedAt) ||
		!rotation.DrainDeadline.After(rotation.CreatedAt) ||
		rotation.Deadline.Sub(rotation.CreatedAt) > MaxOperationLifetime {
		return ErrCorruptState
	}
	if _, err := hex.DecodeString(rotation.PayloadHash); err != nil {
		return ErrCorruptState
	}
	channelIndex := channelRecordIndex(state, rotation.ChannelID)
	if channelIndex < 0 {
		return ErrCorruptState
	}
	if err := validateMembershipChangeRecord(rotation); err != nil {
		return err
	}
	if rotation.Renewal && rotation.MembershipChange.Version != 0 {
		return ErrCorruptState
	}
	if len(rotation.FenceEvidence) > MaxMembersPerChannel+1 {
		return ErrCorruptState
	}
	fenced := make(map[string]struct{}, len(rotation.FenceEvidence))
	for _, evidence := range rotation.FenceEvidence {
		if evidence.RealmID != state.RealmID ||
			evidence.OperationID != rotation.OperationID ||
			validateStoredFenceEvidence(evidence) != nil {
			return ErrCorruptState
		}
		if !fenceTargetAllowed(state, rotation, evidence.TargetPrincipal) {
			return ErrCorruptState
		}
		if _, duplicate := fenced[evidence.TargetPrincipal]; duplicate {
			return ErrCorruptState
		}
		fenced[evidence.TargetPrincipal] = struct{}{}
	}
	switch rotation.Phase {
	case DeliveryPhaseDelivering, DeliveryPhaseInstalled:
		if len(rotation.Activation.Signature) != 0 || rotation.CompletionSequence != 0 {
			return ErrCorruptState
		}
	case DeliveryPhaseActivationCommitted:
		activationPublic, ok := authorityPublicForPrincipal(
			state, rotation.Activation.AuthorityPrincipal,
		)
		if !ok {
			return ErrCorruptState
		}
		if identitycapability.VerifyGenerationActivation(
			rotation.Activation, activationPublic,
		) != nil ||
			rotation.Activation.OperationID != rotation.OperationID ||
			rotation.Activation.ChannelID != rotation.ChannelID ||
			rotation.Activation.Generation != rotation.PendingGeneration ||
			rotation.Activation.PreviousGeneration != rotation.PreviousGeneration ||
			rotation.CompletionSequence != 0 {
			return ErrCorruptState
		}
	case DeliveryPhaseCompleted:
		activationPublic, ok := authorityPublicForPrincipal(
			state, rotation.Activation.AuthorityPrincipal,
		)
		if !ok {
			return ErrCorruptState
		}
		if identitycapability.VerifyGenerationActivation(
			rotation.Activation, activationPublic,
		) != nil ||
			rotation.Activation.OperationID != rotation.OperationID ||
			rotation.Activation.ChannelID != rotation.ChannelID ||
			rotation.Activation.Generation != rotation.PendingGeneration ||
			rotation.Activation.PreviousGeneration != rotation.PreviousGeneration ||
			rotation.CompletionSequence <= rotation.Activation.AuthoritySequence ||
			rotation.CompletionSequence > state.AuthoritySequence {
			return ErrCorruptState
		}
	default:
		return ErrCorruptState
	}
	seen := make(map[string]struct{}, len(rotation.DeliveryIDs))
	for _, deliveryID := range rotation.DeliveryIDs {
		if !deliveryIDPattern.MatchString(deliveryID) {
			return ErrCorruptState
		}
		if _, duplicate := seen[deliveryID]; duplicate {
			return ErrCorruptState
		}
		seen[deliveryID] = struct{}{}
		index := deliveryRecordIndex(state, deliveryID)
		if index < 0 ||
			state.InitialGenerationDeliveries[index].OperationID != rotation.OperationID ||
			string(state.InitialGenerationDeliveries[index].Sealed.Binding.ChannelClass) !=
				state.Channels[channelIndex].Class {
			return ErrCorruptState
		}
	}
	if rotation.Phase == DeliveryPhaseCompleted &&
		!membershipCompletionSatisfied(state, rotation) {
		return ErrCorruptState
	}
	return nil
}

func validateMembershipChangeRecord(rotation RotationRecord) error {
	change := rotation.MembershipChange
	if change.Version == 0 {
		if change != (MembershipChangeRecord{}) || len(rotation.FenceEvidence) != 0 {
			return ErrCorruptState
		}
		return nil
	}
	if change.Version != ContractVersion ||
		(change.Kind != MembershipChangeAdd && change.Kind != MembershipChangeRemove) ||
		change.TargetPrincipal == "" ||
		change.MembershipVersion != rotation.PrepareSequence {
		return ErrCorruptState
	}
	if change.Kind == MembershipChangeAdd {
		if change.PriorState != MemberStateRemoved ||
			change.PendingState != MemberStateCandidate ||
			change.TerminalState != MemberStateActive {
			return ErrCorruptState
		}
	} else if change.PriorState != MemberStateActive ||
		change.PendingState != MemberStateSuspended ||
		change.TerminalState != MemberStateRemoved {
		return ErrCorruptState
	}
	expectedState := change.PendingState
	if rotation.Phase == DeliveryPhaseCompleted {
		expectedState = change.TerminalState
	}
	if change.State != expectedState {
		return ErrCorruptState
	}
	return nil
}
