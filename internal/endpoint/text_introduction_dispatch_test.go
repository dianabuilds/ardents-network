//go:build linux

package endpoint

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func (owner *textContext) acceptTextIntroduction(ctx context.Context, job *textJobIdentity,
	operation []byte) (*textIntroductionAttempt, error) {
	return owner.acceptTextIntroductionGeneration(ctx, job, operation, nil, 1, time.Time{}, false)
}

func (owner *textContext) acceptTextRecovery(ctx context.Context, job *textJobIdentity, operation []byte,
	original *textServiceBinding, request nativeconnection.Recovery) (*textIntroductionAttempt, error) {
	if original == nil || original.owner != owner || original.job != job {
		return nil, errors.New("text recovery binding unavailable")
	}
	if err := original.validateTextServiceRecovery(request); err != nil {
		return nil, err
	}
	return owner.acceptTextIntroductionGeneration(ctx, job, operation, original, request.Generation, request.Deadline, false)
}

// holdTextInitialIntroductionReceiver reproduces the production Publisher
// loop waiting for another initial Connection while an established Connection
// needs a recovery capsule. The returned cleanup is registered before return.
func holdTextInitialIntroductionReceiver(t *testing.T, ctx context.Context, owner *textContext,
	job *textJobIdentity) func() {
	t.Helper()
	waiting, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		attempt, err := owner.receiveTextIntroduction(waiting, job)
		if attempt != nil {
			err = errors.Join(err, errors.New("initial receiver consumed a recovery delivery"))
		}
		done <- err
	}()
	for {
		owner.mu.Lock()
		gate := owner.introductionDelivery
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
