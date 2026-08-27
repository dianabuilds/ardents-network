package custody

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
)

const (
	// OperationCreateVaultRecord creates one independently encrypted Vault record.
	OperationCreateVaultRecord OperationKind = "create-vault-record"
	// OperationVerifyVaultRecord authenticates one record without releasing its root material.
	OperationVerifyVaultRecord OperationKind = "verify-vault-record"
	// OperationExportRecoveryBundle creates and isolatedly test-restores one new Bundle.
	OperationExportRecoveryBundle OperationKind = "export-recovery-bundle"
	// OperationRestoreRecoveryBundle imports a Bundle only as an authority-locked record.
	OperationRestoreRecoveryBundle OperationKind = "restore-recovery-bundle"
	// OperationSignNamespaceTransition signs one sealed Namespace transition with
	// an active Name Authority record without releasing its root material.
	OperationSignNamespaceTransition OperationKind = "sign-namespace-transition"
	// OperationPrepareNamespaceSubmission derives and signs one complete
	// existing-Name control submission without releasing its root material.
	OperationPrepareNamespaceSubmission OperationKind = "prepare-namespace-submission"
	// OperationActivateRecoveredAuthority accepts a strictly newer authenticated
	// current Namespace witness and converts one authority-locked record into a
	// new active Vault successor before any signing operation can run.
	OperationActivateRecoveredAuthority OperationKind = "activate-recovered-authority"
	// OperationPurgeVaultRecord destroys one exact encrypted active or locked
	// record only after password verification and an explicit confirmation. It
	// deliberately retains Authority floors.
	OperationPurgeVaultRecord OperationKind = "purge-vault-record"
	// OperationInspectEnvelope validates only an envelope's public canonical header.
	OperationInspectEnvelope OperationKind = "inspect-envelope"
)

var (
	// ErrClosed reports an operation after the owning Vault has been closed.
	ErrClosed = errors.New("custody vault closed")
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
}

// OperationKind selects one custody state transition. No unrecognized operation
// is accepted.
type OperationKind string

// Operation supplies the bounded data for exactly one Kind. Path is a public
// envelope source for inspection and an Owner-selected new Bundle destination
// for export. Fields unrelated to the selected operation must retain their zero
// value.
type Operation struct {
	Kind           OperationKind
	Authority      AuthorityState
	RecordID       string
	Expected       AuthorityBinding
	Path           string
	Transition     NamespaceTransition
	Preparation    NamespaceSubmission
	Reconciliation *epoch.NameAuthorityReconciliation
}

// NamespaceTransition invokes one sealed Namespace signer and returns its
// public transition proof. It may not manufacture a signing request: Namespace
// owns the exact request construction.
type NamespaceTransition func(authority.TransitionSigner) ([]byte, error)

// NamespaceSubmission invokes Namespace's control-preparation seam. Namespace
// derives the exact transition and successor Record; custody only supplies the
// paired Authority operations and returns the resulting opaque Submission.
type NamespaceSubmission func(authority.ControlSigner) (authority.Submission, error)

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
	Operation    OperationKind
	RecordID     string
	Envelope     EnvelopeInfo
	Authority    AuthorityReceipt
	TestRestored bool
	State        RecordState
	Proof        []byte
	Submission   []byte
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
