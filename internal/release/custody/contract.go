package custody

import (
	"context"
	"errors"
)

var (
	// ErrInvalid reports malformed, unsafe, or unsupported initialization input.
	ErrInvalid = errors.New("release custody input invalid")
	// ErrExists reports an existing seed record. Initialization never replaces it.
	ErrExists = errors.New("release custody seed record already exists")
	// ErrSecret reports a rejected secret while opening encrypted test material.
	ErrSecret = errors.New("release custody secret rejected")
	// ErrPasswordLength reports a passphrase outside the fixed accepted length.
	ErrPasswordLength = errors.New("release custody passphrase must contain 16 to 1024 bytes")
	// ErrConfirmation reports two distinct local passphrase entries.
	ErrConfirmation = errors.New("release custody passphrase confirmation does not match")
)

// InitializeConfig identifies an existing owner-only directory outside the
// repository. The module writes exactly one encrypted seed record there.
type InitializeConfig struct {
	Root string
}

// SecretInput obtains an explicit local secret without accepting it from an
// argument, environment, configuration file, or shared application stream.
type SecretInput interface {
	ReadSecret(context.Context, Prompt) ([]byte, error)
}

// Prompt tells a trusted terminal adapter why it is asking for a secret.
type Prompt string

const (
	PromptCreate  Prompt = "release-custody-create"
	PromptConfirm Prompt = "release-custody-confirm"
	// PromptUnlock asks for the existing record passphrase only to derive a
	// public receipt. It never authorizes arbitrary signing or key export.
	PromptUnlock Prompt = "release-custody-unlock"
)

// PublicRole identifies one fixed ceremony role and its Ed25519 public key.
type PublicRole struct {
	Role   string
	Public [32]byte
}

// Receipt contains no private material. EnvelopeDigest covers the exact
// encrypted record bytes retained at Root.
type Receipt struct {
	EnvelopeDigest [32]byte
	Roles          [10]PublicRole
}

// InspectConfig identifies an existing owner-only directory containing the
// encrypted record. Inspect returns its public receipt only after the supplied
// local secret successfully opens the record.
type InspectConfig struct {
	Root string
}
