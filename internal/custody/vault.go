package custody

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Vault is the exclusive owner of one encrypted record root.
type Vault struct {
	root       string
	records    string
	quarantine string
	floors     string
	now        func() time.Time
	mu         sync.Mutex
	closed     bool
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
	if err := prepareVaultLock(root); err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	records := filepath.Join(root, "records")
	if err := os.MkdirAll(records, 0o700); err != nil {
		return nil, fmt.Errorf("create record root: %w", err)
	}
	quarantine := filepath.Join(root, "quarantine")
	if err := os.MkdirAll(quarantine, 0o700); err != nil {
		return nil, fmt.Errorf("create quarantine record root: %w", err)
	}
	return &Vault{root: root, records: records, quarantine: quarantine,
		floors: filepath.Join(root, "authority-floors.json"), now: now}, nil
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
