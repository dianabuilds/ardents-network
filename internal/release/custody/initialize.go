package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
)

var roleNames = [...]string{
	"tuf-top-level-1",
	"tuf-top-level-2",
	"tuf-top-level-3",
	"tuf-top-level-4",
	"tuf-top-level-5",
	"alpha-disclosure",
	"alpha-release-component",
	"alpha-network-component",
	"alpha-compatibility-component",
	"alpha-corpus-authority",
}

// Initialize creates one fixed encrypted local seed record. It refuses to
// replace an existing record and retains no decrypted material after return.
func Initialize(ctx context.Context, config InitializeConfig, secrets SecretInput) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	root, err := checkedRoot(config.Root)
	if err != nil {
		return Receipt{}, err
	}
	if err := requireAbsent(seedPath(root)); err != nil {
		return Receipt{}, err
	}
	if secrets == nil {
		return Receipt{}, ErrInvalid
	}
	password, err := secrets.ReadSecret(ctx, PromptCreate)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	confirmation, err := secrets.ReadSecret(ctx, PromptConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(confirmation)
	if !validPassword(password) || subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return Receipt{}, ErrSecret
	}

	var record seedRecord
	record.Schema = "ardents-release-seed-record-v1"
	var receipt Receipt
	for index, role := range roleNames {
		public, private, generateErr := ed25519.GenerateKey(rand.Reader)
		if generateErr != nil {
			zeroRecord(record)
			return Receipt{}, fmt.Errorf("generate %s key: %w", role, generateErr)
		}
		var publicValue [ed25519.PublicKeySize]byte
		copy(publicValue[:], public)
		record.Roles[index] = seedRole{Role: role, Private: append([]byte(nil), private...)}
		receipt.Roles[index] = PublicRole{Role: role, Public: publicValue}
		zero(private)
	}
	plaintext, err := marshalRecord(record)
	zeroRecord(record)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintext)
	envelope, err := seal(plaintext, password)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(envelope)
	if err := writeNew(seedPath(root), envelope); err != nil {
		return Receipt{}, err
	}
	receipt.EnvelopeDigest = digest(envelope)
	return receipt, nil
}
