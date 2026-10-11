package custody

import (
	"context"
	"errors"
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
	serial     chan struct{}
	active     *vaultOperation
	closeDone  chan struct{}
	stopping   chan struct{}
	closeErr   error
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
		floors: filepath.Join(root, "authority-floors.json"), now: now, serial: make(chan struct{}, 1), closeDone: make(chan struct{}), stopping: make(chan struct{})}, nil
}

// Close synchronously denies new work, cancels the original operation, and
// physically joins it before returning. It never holds the state mutex on I/O.
func (vault *Vault) Close() error {
	vault.mu.Lock()
	if vault.closed {
		done := vault.closeDone
		vault.mu.Unlock()
		<-done
		return vault.closeErr
	}
	vault.closed = true
	close(vault.stopping)
	op := vault.active
	if op != nil {
		op.cancel(ErrClosed)
	}
	vault.mu.Unlock()
	var err error
	if op != nil {
		<-op.done
		err = op.err
	}
	vault.mu.Lock()
	vault.closeErr = err
	close(vault.closeDone)
	vault.mu.Unlock()
	return err
}

type vaultOperation struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
	err    error
}

// begin serializes original users. Close can revoke while another owner is
// executing terminal/storage work; the original user alone finishes and erases.
func (vault *Vault) begin(ctx context.Context) (context.Context, func(error) error, error) {
	select {
	case vault.serial <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, context.Cause(ctx)
	case <-vault.stopping:
		return nil, nil, ErrClosed
	}
	vault.mu.Lock()
	if vault.closed {
		vault.mu.Unlock()
		<-vault.serial
		return nil, nil, ErrClosed
	}
	original, cancel := context.WithCancelCause(ctx)
	op := &vaultOperation{ctx: original, cancel: cancel, done: make(chan struct{})}
	vault.active = op
	vault.mu.Unlock()
	return original, func(result error) error {
		vault.mu.Lock()
		err := context.Cause(original)
		op.err = errors.Join(result, err)
		vault.active = nil
		cancel(context.Canceled)
		close(op.done)
		vault.mu.Unlock()
		<-vault.serial
		return op.err
	}, nil
}

// check admits a local effect on the same operation; filesystem work remains
// serialized by the operation, and is never performed under mu.
func (vault *Vault) check() error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.closed {
		return ErrClosed
	}
	if vault.active == nil {
		return ErrInvalid
	}
	return context.Cause(vault.active.ctx)
}
