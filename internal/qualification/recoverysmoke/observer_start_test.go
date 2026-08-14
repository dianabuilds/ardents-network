package recoverysmoke

import (
	"context"
	"errors"
	"testing"
)

func TestRetryObserverStartUsesExactRunningPostcondition(t *testing.T) {
	transient := errors.New("transient runtime start")
	attempts, inspections := 0, 0
	err := retryObserverStart(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return transient
		}
		return nil
	}, func(context.Context) (bool, error) {
		inspections++
		return inspections >= 3, nil
	})
	if err != nil || attempts != 3 || inspections != 3 {
		t.Fatalf("bounded retry = attempts %d inspections %d error %v", attempts, inspections, err)
	}
}

func TestRetryObserverStartPreservesFailuresAndCancellation(t *testing.T) {
	startErr, inspectErr := errors.New("start failure"), errors.New("inspect failure")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := retryObserverStart(ctx, func(context.Context) error { return startErr },
		func(context.Context) (bool, error) { return false, inspectErr })
	if !errors.Is(err, context.Canceled) || !errors.Is(err, startErr) || !errors.Is(err, inspectErr) {
		t.Fatalf("retry causes are incomplete: %v", err)
	}
}
