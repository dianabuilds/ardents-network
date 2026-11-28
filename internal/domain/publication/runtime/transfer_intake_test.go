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

func TestInitialDeliveryRefusalIsIsolatedOnlyAfterJoinAndLiveAuthority(t *testing.T) {
	work := errors.New("candidate refused")
	cleanup := errors.New("original output failed during cleanup")
	authority := context.Canceled
	for _, test := range []struct {
		name             string
		cleanup, current error
		isolated         bool
	}{
		{name: "only the candidate failed", isolated: true},
		{name: "physical cleanup failed", cleanup: cleanup},
		{name: "original authority revoked", current: authority},
		{name: "cleanup and authority failed", cleanup: cleanup, current: authority},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := initialDeliveryFailure(work, test.cleanup, test.current)
			if errors.Is(err, errInitialDeliveryRejected) != test.isolated {
				t.Fatal("candidate isolation changed the original Publisher lifetime", err)
			}
			for _, original := range []error{work, test.cleanup, test.current} {
				if original != nil && !errors.Is(err, original) {
					t.Fatal("candidate isolation discarded an original result", err)
				}
			}
		})
	}
}
