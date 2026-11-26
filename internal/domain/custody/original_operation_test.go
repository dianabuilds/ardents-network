package custody

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

type heldTerminal struct {
	started, interrupted, joined chan struct{}
	password                     []byte
}

func (input *heldTerminal) ReadSecret(ctx context.Context, _ SecretPrompt) ([]byte, error) {
	close(input.started)
	<-ctx.Done()
	close(input.interrupted)
	<-input.joined
	return input.password, nil // late success must still be erased and refused
}
func (*heldTerminal) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	return false, ErrInvalid
}

func TestCloseInterruptsAndJoinsOriginalTerminalBeforeLeaseReturn(t *testing.T) {
	checkCloseOriginalTerminal(t, OperationCreateAdmissionAuthority, AuthorityAdmission)
}

func TestServiceCloseInterruptsAndJoinsOriginalTerminalBeforeLeaseReturn(t *testing.T) {
	checkCloseOriginalTerminal(t, OperationCreateServiceAuthority, AuthorityService)
}

func TestServiceIssuanceCancellationJoinsUnlockBeforeAnySuccessor(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	floor, err := os.ReadFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	input := &heldTerminal{make(chan struct{}), make(chan struct{}), make(chan struct{}), bytes.Clone(f.password)}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := f.vault.Execute(ctx, op, input); result <- err }()
	<-input.started
	cancel()
	<-input.interrupted
	select {
	case err := <-result:
		t.Fatal("issuance returned before physical unlock join", err)
	default:
	}
	close(input.joined)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal("late unlock replaced original cancellation", err)
	}
	if !bytes.Equal(input.password, make([]byte, len(input.password))) {
		t.Fatal("late signing password retained")
	}
	after, err := os.ReadFile(f.vault.floors)
	entries, listErr := os.ReadDir(f.vault.records)
	if err != nil || listErr != nil || !bytes.Equal(floor, after) || len(entries) != 1 {
		t.Fatal("cancelled issuance committed successor", err, listErr)
	}
}

func checkCloseOriginalTerminal(t *testing.T, kind OperationKind, authority AuthorityKind) {
	t.Helper()
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	input := &heldTerminal{make(chan struct{}), make(chan struct{}), make(chan struct{}), []byte("held original terminal password")}
	result := make(chan error, 1)
	go func() {
		_, err := vault.Execute(t.Context(), Operation{Kind: kind, Binding: AuthorityBinding{Kind: authority, Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}}}, input)
		result <- err
	}()
	<-input.started
	closed := make(chan error, 1)
	go func() { closed <- vault.Close() }()
	<-input.interrupted
	if _, err := vault.Execute(t.Context(), Operation{}, &sequenceSecrets{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("new effects after Close: %v", err)
	}
	select {
	case err := <-closed:
		t.Fatalf("Close preceded original join: %v", err)
	default:
	}
	if lock, err := acquireVaultOperationLock(vault.root); !errors.Is(err, ErrBusy) {
		if lock != nil {
			_ = lock.release()
		}
		t.Fatalf("lease returned before join: %v", err)
	}
	close(input.joined)
	if err := <-result; !errors.Is(err, ErrClosed) {
		t.Fatalf("late password success: %v", err)
	}
	if err := <-closed; !errors.Is(err, ErrClosed) {
		t.Fatalf("Close lost original refusal: %v", err)
	}
	if !bytes.Equal(input.password, make([]byte, len(input.password))) {
		t.Fatal("late password retained")
	}
	entries, err := os.ReadDir(vault.records)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled creation wrote record: %v / %v", entries, err)
	}
	lock, err := acquireVaultOperationLock(vault.root)
	if err != nil {
		t.Fatalf("joined lease not returned: %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatal(err)
	}
}
