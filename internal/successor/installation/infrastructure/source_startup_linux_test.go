package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These controls exercise incomplete lifetime retirement only. They neither
// open an installed Source nor substitute an accepting process/manager proof.
func TestSourceStartupStopDeniesWithoutWaitingForNativeObserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	o := &sourceStartup{ctx: ctx, parent: context.Background(), cancel: cancel}
	s := &SourceStartup{original: o}
	copy := *s
	o.ioMu.Lock() // represent an already admitted observation held in native I/O
	defer o.ioMu.Unlock()
	done := make(chan struct{})
	go func() { copy.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("local Stop waited for observation I/O")
	}
	if !errors.Is(ctx.Err(), context.Canceled) || !errors.Is(o.begin(), context.Canceled) {
		t.Fatal("copied handle did not retire the original admission")
	}
	if err := s.Check(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Check reached incomplete native owners after Stop: %v", err)
	}
	if _, err := copy.Inputs(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Inputs reached incomplete native owners after Stop: %v", err)
	}
}

func TestSourceStartupCloseRetainsFilesUntilOriginalBorrowJoins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	file, err := os.Create(filepath.Join(t.TempDir(), "retained-input"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	raw := []byte("private lifetime control")
	o := &sourceStartup{ctx: ctx, parent: context.Background(), cancel: cancel,
		selection: &sourceSelection{files: map[string]*sourceFile{"input": {file: file, raw: raw}}}}
	s := &SourceStartup{original: o}
	copy := *s
	// Acquire only a local borrow; no successful native Check is manufactured.
	if err := o.begin(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- copy.Close() }()
	select {
	case <-ctx.Done(): // Close has denied/interrupting admission before joining
	case <-time.After(time.Second):
		t.Fatal("Close did not interrupt original users")
	}
	select {
	case err := <-done:
		t.Fatalf("Close returned before original borrower joined: %v", err)
	default:
	}
	if _, err := file.Stat(); err != nil || string(raw) != "private lifetime control" {
		t.Fatalf("original files or bytes released during borrow: %v", err)
	}
	// A real descriptor-close failure must survive the joined terminal outcome.
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	o.uses.Done()
	var result error
	select {
	case result = <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not join after borrower completed")
	}
	if !errors.Is(result, os.ErrClosed) {
		t.Fatalf("physical close error lost: %v", result)
	}
	for _, value := range raw {
		if value != 0 {
			t.Fatal("joined Close retained private bytes")
		}
	}
	if err := s.Close(); err != result || copy.Close() != result {
		t.Fatal("copies did not retain the same physical Close result")
	}
}

func TestSourceStartupOriginalFailureCannotBeRevived(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o := &sourceStartup{ctx: ctx, parent: context.Background(), cancel: cancel}
	s := &SourceStartup{original: o}
	first := errors.New("original selected input lost")
	if o.record(first) != first || !errors.Is(s.Check(), first) {
		t.Fatal("original failure was replaced by cancellation")
	}
	if _, err := s.Inputs(); !errors.Is(err, first) {
		t.Fatalf("failed lifetime supplied detached inputs: %v", err)
	}
	if err := s.Close(); !errors.Is(err, first) {
		t.Fatalf("Close lost original refusal: %v", err)
	}
}
