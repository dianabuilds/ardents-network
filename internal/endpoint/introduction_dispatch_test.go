//go:build linux

package endpoint

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func (owner *dutyContext) acceptIntroduction(ctx context.Context, job *jobIdentity,
	operation []byte) (*introductionAttempt, error) {
	return owner.acceptIntroductionGeneration(ctx, job, operation, nil, 1, time.Time{}, false)
}

func (owner *dutyContext) acceptRecovery(ctx context.Context, job *jobIdentity, operation []byte,
	original *serviceBinding, request nativeconnection.Recovery) (*introductionAttempt, error) {
	if original == nil || original.owner != owner || original.job != job {
		return nil, errors.New("text recovery binding unavailable")
	}
	if err := original.validateServiceRecovery(request); err != nil {
		return nil, err
	}
	return owner.acceptIntroductionGeneration(ctx, job, operation, original, request.Generation, request.Deadline, false)
}

// holdInitialIntroductionReceiver reproduces the production Publisher
// loop waiting for another initial Connection while an established Connection
// needs a recovery capsule. The returned cleanup is registered before return.
func holdInitialIntroductionReceiver(t *testing.T, ctx context.Context, owner *dutyContext,
	job *jobIdentity) func() {
	t.Helper()
	waiting, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		attempt, err := owner.receiveIntroduction(waiting, job)
		if attempt != nil {
			err = errors.Join(err, errors.New("initial receiver consumed a recovery delivery"))
		}
		done <- err
	}()
	for {
		owner.mu.Lock()
		gate := introduction.ConsumerGate(&owner.introduction.dispatch)
		owner.mu.Unlock()
		if gate != nil && len(gate) == 0 {
			break
		}
		select {
		case err := <-done:
			cancel()
			t.Fatalf("initial receiver ended before recovery: %v", err)
		case <-ctx.Done():
			cancel()
			t.Fatal(ctx.Err())
		default:
			runtime.Gosched()
		}
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Errorf("initial receiver cleanup: %v", err)
			}
		})
	}
	t.Cleanup(stop)
	return stop
}
