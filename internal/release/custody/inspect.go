package custody

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
)

// Inspect opens one encrypted record solely to confirm its passphrase and
// derive the fixed public role receipt. It does not expose private material or
// provide a signing operation.
func Inspect(ctx context.Context, config InspectConfig, secrets SecretInput) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	root, err := checkedRoot(config.Root)
	if err != nil {
		return Receipt{}, err
	}
	if secrets == nil {
		return Receipt{}, ErrInvalid
	}
	password, err := secrets.ReadSecret(ctx, PromptUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	if !validPassword(password) {
		return Receipt{}, ErrSecret
	}
	raw, err := readRecord(seedPath(root))
	if err != nil {
		return Receipt{}, err
	}
	defer zero(raw)
	record, err := openRecord(raw, password)
	if err != nil {
		return Receipt{}, err
	}
	defer zeroRecord(record)
	return receiptFor(record, raw), nil
}

func readRecord(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read release custody seed record: %w", err)
	}
	return raw, nil
}

func receiptFor(record seedRecord, raw []byte) Receipt {
	result := Receipt{EnvelopeDigest: digest(raw)}
	for index, role := range roleNames {
		public := ed25519.PrivateKey(record.Roles[index].Private).Public().(ed25519.PublicKey)
		result.Roles[index].Role = role
		copy(result.Roles[index].Public[:], public)
	}
	return result
}
