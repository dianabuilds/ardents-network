package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestTransferIntakeDrainSealsAdmissionBeforeJoiningActiveWork(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	intake := &TransferIntake{ctx: ctx, cancel: cancel, stopped: make(chan struct{})}
	intake.flights.Add(1)
	joined := make(chan struct{})
	go func() {
		intake.Drain()
		close(joined)
	}()
	<-intake.stopped
	if !errors.Is(intake.checkCapacity(), errTransferUnavailable) || ctx.Err() != nil {
		t.Error("withdrawal admitted another transfer or canceled active work")
	}
	select {
	case <-joined:
		t.Error("withdrawal returned before active work joined")
	default:
	}
	intake.flights.Done()
	<-joined
	if ctx.Err() != context.Canceled {
		t.Fatal("joined withdrawal retained its admission lifetime")
	}
}
