package infrastructure

import (
	"context"
	"errors"
	"testing"
)

// Cancellation can arrive after check admits its original native observation
// and before managerStart returns the same process clock. The context retains
// that exact ordering without replacing the native files or manager facts.
type sourceIdentityCancelAfterAdmission struct {
	context.Context
	cancel context.CancelFunc
}

func (c *sourceIdentityCancelAfterAdmission) Err() error {
	err := c.Context.Err()
	c.cancel()
	return err
}

func TestSourceRuntimeIdentityCancellationDuringManagerStart(t *testing.T) {
	identity, err := openSourceManagerIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer identity.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	identity.ctx = &sourceIdentityCancelAfterAdmission{Context: ctx, cancel: cancel}
	err = identity.check()
	if !errors.Is(err, context.Canceled) || !sourceJoinedCancellation(err) {
		t.Fatalf("ordinary cancellation became a failed original binding: %v", err)
	}
}

func TestSourceRuntimeIdentityForeignManagerClockRemainsFailure(t *testing.T) {
	identity, err := openSourceManagerIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer identity.close()
	identity.started++
	if err := identity.check(); !errors.Is(err, ErrBinding) || sourceJoinedCancellation(err) {
		t.Fatalf("foreign original manager clock was not retained as failure: %v", err)
	}
}

func TestSourceRuntimeIdentityForeignManagerClockAndCancellation(t *testing.T) {
	identity, err := openSourceManagerIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer identity.close()
	identity.started++
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	identity.ctx = &sourceIdentityCancelAfterAdmission{Context: ctx, cancel: cancel}
	err = identity.check()
	if !errors.Is(err, ErrBinding) || !errors.Is(err, context.Canceled) || sourceJoinedCancellation(err) {
		t.Fatalf("observed foreign manager clock or cancellation was lost: %v", err)
	}
}
