package authority

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	identitycapability "ardents/internal/identity/capability"
)

func validateLedger(state Ledger) error {
	if state.Version != ContractVersion || state.SchemaVersion != SchemaVersion ||
		state.Revision == 0 || !ValidRealmID(state.RealmID) ||
		state.RealmClass != RealmClassProduction || state.AuthorityEpoch != 1 ||
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
	for _, channel := range state.Channels {
		if channel.MemberCount > MaxMembersPerChannel ||
			channel.PendingGenerationCount > MaxPendingGenerations ||
			channel.PreviousReceiveGenerationCount > MaxPreviousReceiveGenerations ||
			channel.OutstandingDeliveryCount > MaxOutstandingDeliveries*channel.MemberCount {
			return corruptLedger("channel bounds")
		}
		grant, ok := channel.Grant.restore()
		if !ok || channel.ID != grant.ChannelID ||
			channel.Class != string(grant.Scope) ||
			channel.CurrentGeneration != grant.Generation ||
			channel.MemberCount == 0 ||
			identitycapability.VerifyGrant(grant, ed25519.PublicKey(state.AuthorityPublicKey)) != nil {
			return corruptLedger("channel grant")
		}
	}
	for _, member := range state.Members {
		if member.Version != ContractVersion || member.Principal == "" {
			return corruptLedger("member")
		}
	}
	for _, operation := range state.Operations {
		if operation.Version != ContractVersion || !operationIDPattern.MatchString(operation.ID) ||
			len(operation.RequestID) == 0 || len(operation.RequestID) > MaxRequestIDBytes ||
			operation.Kind != "realm_genesis" || !canonicalSecond(operation.CreatedAt) ||
			!canonicalSecond(operation.Deadline) || !operation.Deadline.After(operation.CreatedAt) ||
			operation.Deadline.Sub(operation.CreatedAt) > MaxOperationLifetime ||
			(operation.Phase != PhaseCheckpointing && operation.Phase != PhaseReady && operation.Phase != PhaseRecoveryRequired) {
			return corruptLedger("genesis operation")
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
			record.Result.AuthorityEpoch != state.AuthorityEpoch ||
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
	if err := ValidateCheckpoint(state.Checkpoint); err != nil ||
		state.Checkpoint.RealmID != state.RealmID ||
		state.Checkpoint.AuthoritySequence != state.AuthoritySequence ||
		state.Checkpoint.Digest == "" {
		return corruptLedger("checkpoint")
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
	state.Members = append([]MemberRecord(nil), state.Members...)
	state.Channels = append([]ChannelRecord(nil), state.Channels...)
	for index := range state.Channels {
		state.Channels[index].Grant.Secret = append([]byte(nil), state.Channels[index].Grant.Secret...)
		state.Channels[index].Grant.Signature = append([]byte(nil), state.Channels[index].Grant.Signature...)
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
	switch record.Action {
	case ActionCreate:
		if record.ResourceKind != ResourceKindAuthorityInstance ||
			record.ResourceID != PrimaryAuthorityInstance {
			return ErrCorruptState
		}
	case ActionIssueDelivery, ActionAcknowledgeDelivery:
		if record.ResourceKind != ResourceKindGenerationDelivery ||
			!validGenerationDeliveryResource(record.ResourceID) {
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
	return nil
}
