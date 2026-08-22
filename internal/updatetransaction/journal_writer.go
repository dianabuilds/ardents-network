package updatetransaction

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

func (trace *tracer) record(ctx context.Context, name string, state transactionState, adapter adapterResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := applyCheckpoint(trace.control, true, name); err != nil {
		return err
	}
	entry := journalEntry{State: state, Generation: trace.request.Generation, Predecessor: trace.predecessor,
		ArtifactDigest: trace.artifact, ManifestCommitment: trace.manifest, AdapterResult: adapter,
		Observation: byte(state), ElapsedNanos: trace.elapsedOffset + uint64(time.Since(trace.start)),
		DeadlineUnix: trace.journalDeadline(state).Unix()}
	raw, err := trace.store.writeEntry(entry)
	if err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	trace.predecessor = sha256.Sum256(raw)
	return applyCheckpoint(trace.control, false, name)
}

func applyCheckpoint(control *applyInterruptionControl, before bool, name string) error {
	if control == nil {
		return nil
	}
	stop := control.StopAfter
	if before {
		stop = control.StopBefore
	}
	if stop != nil && stop(name) {
		return errApplyInterrupted
	}
	return nil
}
