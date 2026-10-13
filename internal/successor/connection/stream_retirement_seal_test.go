package connection

import (
	"context"
	"errors"
	"testing"
)

// These local retirement states supply no successful authentication or
// Application outcome. They isolate denial before record interruption.
func TestStreamRetirementSealPrecedesLateRecordFailure(t *testing.T) {
	// Deterministically schedule the old cancellation-before-seal window.
	// A settled state alone cannot stop failRecord from retaining cancellation.
	before, cancelBefore := context.WithCancel(t.Context())
	unsealed := &Stream{ctx: before, cancel: cancelBefore, settled: true}
	oldRecords := &attachmentRecords{}
	unsealed.records = oldRecords
	cancelBefore()
	unsealed.failRecord(oldRecords, context.Canceled)
	if !errors.Is(unsealed.failure, context.Canceled) {
		t.Fatal("control did not reproduce the unsealed cancellation window")
	}
	for _, settled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		stream := &Stream{ctx: ctx, cancel: cancel, settled: settled}
		records := &attachmentRecords{}
		stream.records = records
		stream.sealRetirement()
		if !stream.closing || ctx.Err() == nil {
			t.Fatal("retirement did not deny effects before interruption")
		}
		lateFailure := errors.New("record read interrupted")
		stream.failRecord(records, lateFailure)
		if settled && stream.failure != nil {
			t.Fatal("late record changed settled logical state", stream.failure)
		}
		if !settled && !errors.Is(stream.failure, context.Canceled) {
			t.Fatal("uncompleted retirement lost cancellation", stream.failure)
		}
		if errors.Is(stream.failure, lateFailure) {
			t.Fatal("late record replaced the earlier retirement cause")
		}
	}
}

func TestStreamRetirementSealRetainsOriginalFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	original := errors.New("original authority refused")
	stream := &Stream{ctx: ctx, cancel: cancel, failure: original}
	stream.sealRetirement()
	stream.failRecord(&attachmentRecords{}, context.Canceled)
	if stream.failure != original {
		t.Fatal("retirement replaced original failure", stream.failure)
	}
}
