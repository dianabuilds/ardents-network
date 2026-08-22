package update

import (
	"context"
	"errors"
	"testing"
)

func TestResumeSuccessfulSelfTest(t *testing.T) {
	_, request := applyCheckpointRequest(t)
	work := request.Work.(*oracleWorkControl)
	calls := 0
	control := &applyInterruptionControl{StopBefore: func(name string) bool {
		if name != "09-committed" {
			return false
		}
		calls++
		return calls == 1
	}}
	if result, err := applyWithInterruption(context.Background(), request, control); !errors.Is(err, errApplyInterrupted) || result != (Result{}) {
		t.Fatalf("interrupted Apply = %+v, %v; checkpoint calls=%d", result, err, calls)
	}

	result, err := Apply(context.Background(), request)
	if err != nil || result.Outcome != "committed" || result.State != "committed" || work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("resumed Apply = %+v, %v; work=%+v", result, err, work)
	}
}
