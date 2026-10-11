package custody

import (
	"context"
	"errors"
	"time"
)

const (
	// OperationCreateServiceAuthority generates a separate offline Authority.
	OperationCreateServiceAuthority OperationKind = "create-service-authority"
	// OperationIssueServiceCredential signs one independently approved request.
	OperationIssueServiceCredential OperationKind = "issue-service-credential"
	// operationCreateVaultRecord is internal encrypted record creation.
	operationCreateVaultRecord OperationKind = "create-vault-record"
	// OperationCreateAdmissionAuthority generates one separate offline
	// permission authority inside a new encrypted Vault record.
	OperationCreateAdmissionAuthority OperationKind = "create-admission-authority"
	// OperationIssueAdmissionPermission signs one exact holder-proof-bound
	// allocation and atomically advances the encrypted allocation ledger.
	OperationIssueAdmissionPermission OperationKind = "issue-admission-permission"
	// OperationVerifyVaultRecord authenticates one record without releasing its root material.
	OperationVerifyVaultRecord OperationKind = "verify-vault-record"
	// OperationExportRecoveryBundle creates and isolatedly test-restores one new Bundle.
	OperationExportRecoveryBundle OperationKind = "export-recovery-bundle"
	// OperationRestoreRecoveryBundle imports a Bundle only as an authority-locked record.
	OperationRestoreRecoveryBundle OperationKind = "restore-recovery-bundle"
	// OperationPurgeVaultRecord destroys one exact encrypted active or locked
	// record only after password verification and an explicit confirmation. It
	// deliberately retains Authority floors.
	OperationPurgeVaultRecord OperationKind = "purge-vault-record"
)

var (
	// ErrClosed reports an operation after the owning Vault has been closed.
	ErrClosed = errors.New("custody vault closed")
	// ErrBusy reports another process currently executing against this Vault.
	ErrBusy = errors.New("custody vault busy")
	// ErrInvalid reports malformed, oversized, or semantically invalid custody input.
	ErrInvalid = errors.New("custody input invalid")
	// ErrUnsupported reports a canonical envelope that selects no supported profile.
	ErrUnsupported = errors.New("custody envelope unsupported")
	// ErrUnlockFailed deliberately combines wrong-password and authenticated-byte failures.
	ErrUnlockFailed = errors.New("custody unlock failed")
)

// VaultConfig selects the exclusive local directory that contains encrypted
// Authority Vault records. It is not a Recovery Bundle destination.
type VaultConfig struct {
	Root string
	// Now supplies the wall clock used for bounded credential issuance. A nil
	// function selects time.Now; tests inject an exact instant.
	Now func() time.Time
}

// OperationKind selects one custody state transition. No unrecognized operation
// is accepted.
type OperationKind string

// Operation contains only public input for a purpose-specific operation.
// AdmissionRequest and ServiceRequest are snapshotted at Execute; callers must
// not mutate them during entry. Binding is used only for independent Admission
// or Service Authority creation; Expected
// selects an exact existing record. No operation accepts private root material.
type Operation struct {
	Kind                       OperationKind
	Binding                    AuthorityBinding
	RecordID                   string
	Expected                   AuthorityBinding
	Path                       string
	AdmissionRequest           []byte
	AdmissionRequestCommitment [32]byte
	ServiceRequest             []byte
	ServiceRequestCommitment   [32]byte
	authority                  authorityState // exclusively generated inside the owner
}

// SecretInput obtains one explicit password entry for the custody boundary.
// Implementations must not source it from argv, environment, configuration, or
// a stdin stream shared with Application data.
type SecretInput interface {
	ReadSecret(context.Context, SecretPrompt) ([]byte, error)
	Confirm(context.Context, ConfirmationPrompt) (bool, error)
}

// SecretPrompt tells a trusted custody-front-end why it is reading a password.
type SecretPrompt string

const (
	SecretPromptVaultCreate         SecretPrompt = "vault-create"
	SecretPromptVaultCreateConfirm  SecretPrompt = "vault-create-confirm"
	SecretPromptVaultUnlock         SecretPrompt = "vault-unlock"
	SecretPromptBundleExport        SecretPrompt = "bundle-export"
	SecretPromptBundleExportConfirm SecretPrompt = "bundle-export-confirm"
	SecretPromptBundleRestore       SecretPrompt = "bundle-restore"
)

// ConfirmationPrompt identifies a non-secret explicit Owner confirmation.
type ConfirmationPrompt string

const (
	ConfirmationPromptBundleReplacement ConfirmationPrompt = "bundle-replacement"
	ConfirmationPromptVaultPurge        ConfirmationPrompt = "vault-purge"
)

// Receipt contains only bounded public custody facts. In particular it never
// includes root material, a password, a derived key, or plaintext bytes.
type Receipt struct {
	Operation           OperationKind
	RecordID            string
	Envelope            EnvelopeInfo
	Authority           AuthorityReceipt
	AdmissionAuthority  AdmissionAuthorityReceipt
	AdmissionPermission []byte
	ServiceAuthority    ServiceAuthorityReceipt
	ServiceResponse     []byte
	TestRestored        bool
	State               RecordState
}

// ServiceAuthorityReceipt contains only the public Authority and its Target.
type ServiceAuthorityReceipt struct {
	Public, Target [32]byte
}

// AdmissionAuthorityReceipt identifies the public key that a signed closed
// profile independently binds to offline issuance permissions.
type AdmissionAuthorityReceipt struct {
	Public [32]byte
}

// RecordState is the non-secret local lifecycle classification of a protected
// record. A restored Bundle begins locked and export-only, never active.
type RecordState string

const (
	RecordActive          RecordState = "active"
	RecordAuthorityLocked RecordState = "authority-locked"
)

// EnvelopeInfo is the public header metadata admitted from one canonical
// envelope. Digest covers the exact canonical envelope bytes.
type EnvelopeInfo struct {
	Purpose        Purpose
	CiphertextSize uint64
	Digest         [32]byte
}

// AuthorityReceipt is the non-secret lifecycle projection of an authenticated
// Authority record.
type AuthorityReceipt struct {
	Binding    AuthorityBinding
	Generation uint64
	Revision   uint64
	Watermarks []Watermark
}

// Purpose separates live Vault records from portable Recovery Bundles in their
// authenticated outer and inner envelope fields.
type Purpose string

const (
	PurposeVault  Purpose = "authority-vault"
	PurposeBundle Purpose = "recovery-bundle"
)
