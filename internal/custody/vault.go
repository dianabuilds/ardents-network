package custody

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Vault is the exclusive owner of one encrypted record root.
type Vault struct {
	root    string
	records string
	mu      sync.Mutex
	closed  bool
}

// Open creates or reopens an encrypted-only Vault root. It does not unlock any
// record and does not accept a password.
func Open(config VaultConfig) (*Vault, error) {
	if config.Root == "" {
		return nil, ErrInvalid
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, fmt.Errorf("vault root: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create vault root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("vault root: %w", ErrInvalid)
	}
	records := filepath.Join(root, "records")
	if err := os.MkdirAll(records, 0o700); err != nil {
		return nil, fmt.Errorf("create record root: %w", err)
	}
	return &Vault{root: root, records: records}, nil
}

// Close rejects future operations. It never leaves a record unlocked because
// every operation derives and discards its key before returning.
func (vault *Vault) Close() error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.closed {
		return ErrClosed
	}
	vault.closed = true
	return nil
}
