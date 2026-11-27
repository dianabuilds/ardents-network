package custody

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Execute performs exactly one bounded custody operation. It never retains a
// password, derived key, plaintext root, or signing capability after return.
func (vault *Vault) Execute(ctx context.Context, operation Operation, secrets SecretInput) (receipt Receipt, resultErr error) {
	if ctx == nil || secrets == nil {
		return Receipt{}, ErrInvalid
	}
	if len(operation.AdmissionRequest) > 512 || len(operation.ServiceRequest) > 1024 {
		return Receipt{}, ErrInvalid
	}
	operation.AdmissionRequest = bytes.Clone(operation.AdmissionRequest)
	operation.ServiceRequest = bytes.Clone(operation.ServiceRequest)
	original, finish, err := vault.begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer func() {
		resultErr = finish(resultErr)
		if resultErr != nil {
			receipt = Receipt{}
		}
	}()
	ctx = original
	if operation.Kind != OperationCreateAdmissionAuthority && operation.Kind != OperationCreateServiceAuthority && operation.Binding != (AuthorityBinding{}) {
		return Receipt{}, ErrInvalid
	}
	operationLock, err := acquireVaultOperationLock(vault.root)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, operationLock.release()) }()
	if operation.Kind != OperationIssueAdmissionPermission && (len(operation.AdmissionRequest) != 0 || operation.AdmissionRequestCommitment != [32]byte{}) {
		return Receipt{}, ErrInvalid
	}
	if operation.Kind != OperationIssueServiceCredential && (len(operation.ServiceRequest) != 0 || operation.ServiceRequestCommitment != [32]byte{}) {
		return Receipt{}, ErrInvalid
	}
	switch operation.Kind {
	case OperationCreateServiceAuthority:
		return vault.createServiceAuthority(ctx, operation, secrets)
	case OperationIssueServiceCredential:
		return vault.issueServiceCredential(ctx, operation, secrets)
	case OperationCreateAdmissionAuthority:
		return vault.createAdmissionAuthority(ctx, operation, secrets)
	case OperationIssueAdmissionPermission:
		return vault.issueAdmissionPermission(ctx, operation, secrets)
	case OperationVerifyVaultRecord:
		return vault.verifyAuthorityRecord(ctx, operation, secrets)
	case OperationExportRecoveryBundle:
		return vault.exportBundle(ctx, operation, secrets)
	case OperationRestoreRecoveryBundle:
		return vault.restoreBundle(ctx, operation, secrets)
	case OperationPurgeVaultRecord:
		return vault.purgeRecord(ctx, operation, secrets)
	default:
		return Receipt{}, ErrInvalid
	}
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

func (vault *Vault) isEmpty() (bool, error) {
	for _, directory := range []string{vault.records, vault.quarantine} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return false, fmt.Errorf("list vault records: %w", err)
		}
		if len(entries) != 0 {
			return false, nil
		}
	}
	entries, err := os.ReadDir(vault.root)
	if err != nil {
		return false, fmt.Errorf("list vault root: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "admission-ledger-") {
			return false, nil
		}
	}
	return true, nil
}
