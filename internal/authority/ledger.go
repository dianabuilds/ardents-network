package authority

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

func validateLedger(state Ledger) error {
	if state.Version != ContractVersion || state.SchemaVersion != SchemaVersion ||
		state.Revision == 0 || !ValidRealmID(state.RealmID) ||
		state.RealmClass != RealmClassProduction || state.AuthorityEpoch != 1 ||
		state.AuthoritySequence != 1 || state.AuthorityPrincipal == "" ||
		len(state.AuthorityPublicKey) != 32 || !digestPattern.MatchString(state.AuditHead) {
		return ErrCorruptState
	}
	if state.Phase != PhaseCheckpointing && state.Phase != PhaseReady && state.Phase != PhaseRecoveryRequired {
		return ErrCorruptState
	}
	switch state.Phase {
	case PhaseCheckpointing:
		if state.Readiness != ReadinessDegraded || state.Reason != ReasonCheckpointMissing {
			return ErrCorruptState
		}
	case PhaseReady:
		if state.Readiness != ReadinessReady || state.Reason != ReasonNone {
			return ErrCorruptState
		}
	case PhaseRecoveryRequired:
		if state.Readiness != ReadinessRecoveryRequired || state.Reason == ReasonNone {
			return ErrCorruptState
		}
	}
	if state.Checkpoint.AuthorityPrincipal != state.AuthorityPrincipal ||
		!equalBytes(state.Checkpoint.AuthorityPublicKey, state.AuthorityPublicKey) ||
		state.Checkpoint.AuditHead != state.AuditHead {
		return ErrCorruptState
	}
	if len(state.Members) > MaxRealmMembers || len(state.Channels) > MaxActiveChannels ||
		len(state.Operations) == 0 || len(state.Operations) > MaxOperations ||
		len(state.Idempotency) == 0 || len(state.Idempotency) > MaxIdempotencyRecords ||
		len(state.AuditLog) == 0 || len(state.AuditLog) > MaxAuditRecords ||
		len(state.AuditOutbox) > MaxAuditOutboxRecords {
		return ErrCorruptState
	}
	for _, channel := range state.Channels {
		if channel.MemberCount > MaxMembersPerChannel ||
			channel.PendingGenerationCount > MaxPendingGenerations ||
			channel.PreviousReceiveGenerationCount > MaxPreviousReceiveGenerations ||
			channel.OutstandingDeliveryCount > MaxOutstandingDeliveries*channel.MemberCount {
			return ErrCorruptState
		}
	}
	for _, operation := range state.Operations {
		if operation.Version != ContractVersion || !operationIDPattern.MatchString(operation.ID) ||
			len(operation.RequestID) == 0 || len(operation.RequestID) > MaxRequestIDBytes ||
			operation.Kind != "realm_genesis" || !canonicalSecond(operation.CreatedAt) ||
			!canonicalSecond(operation.Deadline) || !operation.Deadline.After(operation.CreatedAt) ||
			operation.Deadline.Sub(operation.CreatedAt) > MaxOperationLifetime ||
			(operation.Phase != PhaseCheckpointing && operation.Phase != PhaseReady && operation.Phase != PhaseRecoveryRequired) {
			return ErrCorruptState
		}
	}
	for _, record := range state.Idempotency {
		if record.Version != ContractVersion || len(record.RequestID) == 0 ||
			len(record.RequestID) > MaxRequestIDBytes || len(record.PayloadHash) != sha256.Size*2 ||
			record.Result.Version != ContractVersion || record.Result.RealmID != state.RealmID ||
			record.Result.AuthorityEpoch != state.AuthorityEpoch ||
			record.Result.AuthoritySequence != state.AuthoritySequence ||
			record.Result.CheckpointDigest != state.Checkpoint.Digest ||
			!operationIDPattern.MatchString(record.Result.OperationID) {
			return ErrCorruptState
		}
		if _, err := hex.DecodeString(record.PayloadHash); err != nil {
			return ErrCorruptState
		}
	}
	previousAuditHash := ""
	auditByID := make(map[string]AuditRecord, len(state.AuditLog))
	for _, record := range state.AuditLog {
		if err := validateAuditRecord(record, previousAuditHash); err != nil {
			return ErrCorruptState
		}
		if _, duplicate := auditByID[record.ID]; duplicate {
			return ErrCorruptState
		}
		auditByID[record.ID] = record
		previousAuditHash = record.Hash
	}
	if previousAuditHash != state.AuditHead {
		return ErrCorruptState
	}
	for _, record := range state.AuditOutbox {
		retained, ok := auditByID[record.ID]
		if !ok || retained != record {
			return ErrCorruptState
		}
	}
	if err := ValidateCheckpoint(state.Checkpoint); err != nil ||
		state.Checkpoint.RealmID != state.RealmID ||
		state.Checkpoint.AuthoritySequence != state.AuthoritySequence ||
		state.Checkpoint.Digest == "" {
		return ErrCorruptState
	}
	return nil
}

func cloneLedger(state Ledger) Ledger {
	state.AuthorityPublicKey = append([]byte(nil), state.AuthorityPublicKey...)
	state.Checkpoint.AuthorityPublicKey = append([]byte(nil), state.Checkpoint.AuthorityPublicKey...)
	state.Checkpoint.Signature = append([]byte(nil), state.Checkpoint.Signature...)
	state.Members = append([]MemberRecord(nil), state.Members...)
	state.Channels = append([]ChannelRecord(nil), state.Channels...)
	state.Operations = append([]OperationRecord(nil), state.Operations...)
	state.Idempotency = append([]IdempotencyRecord(nil), state.Idempotency...)
	state.AuditLog = append([]AuditRecord(nil), state.AuditLog...)
	state.AuditOutbox = append([]AuditRecord(nil), state.AuditOutbox...)
	return state
}

func canonicalSecond(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC &&
		value.Nanosecond() == 0
}

func validateAuditRecord(record AuditRecord, previousHash string) error {
	if record.Version != ContractVersion || !auditIDPattern.MatchString(record.ID) ||
		record.Actor == "" || record.Actor != record.Effective || record.Action != ActionCreate ||
		record.ResourceKind != ResourceKindAuthorityInstance ||
		record.ResourceID != PrimaryAuthorityInstance ||
		!operationIDPattern.MatchString(record.OperationID) || record.Outcome != "accepted" ||
		record.PreviousHash != previousHash || !canonicalSecond(record.CreatedAt) ||
		record.Hash != auditHash(record) {
		return ErrCorruptState
	}
	return nil
}
