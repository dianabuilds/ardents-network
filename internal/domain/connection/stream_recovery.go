package connection

import (
	"errors"
	"time"
)

// Stream.mu admits one original interruption/join and clock for both physical
// actors, including simultaneous losses and peer proposals before local loss.
func (stream *Stream) startRecoveryLocked(records *attachmentRecords, cause error) error {
	if stream.records != records || records.generation != stream.transfer.Generation() || (stream.publisherRoute == nil && stream.initiatorRoute == nil) {
		return errIntegrity
	}
	if err := stream.checkLocked(); err != nil {
		return err
	}
	if records.retiring {
		return nil
	}
	if stream.recovery == nil {
		var err error
		stream.recovery, err = newRecoveryEpisode(records.generation, stream.lastProgress, time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
		if err != nil {
			return err
		}
	}
	if err := stream.recovery.start(records.generation, time.Now()); err != nil {
		return err
	}
	records.recoveryCause = cause
	records.retiring = true
	deadline := stream.recovery.deadline
	stream.recoveryUsers.Add(1)
	stream.signalLocked()
	go stream.joinFailedAttachment(records, deadline)
	if stream.initiatorRoute != nil {
		stream.recoveryUsers.Add(1)
		go stream.recoverInitiator(records)
	}
	return nil
}

func (stream *Stream) joinFailedAttachment(records *attachmentRecords, deadline time.Time) {
	defer stream.recoveryUsers.Done()
	go func() {
		var work, physical error
		if records.physical.initiator != nil {
			work = records.physical.initiator.closeFailed()
			physical = records.physical.initiator.cleanupErr
		} else {
			work = records.physical.service.CloseFailed()
			var joined bool
			physical, joined = records.physical.service.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		}
		work = errors.Join(work, records.join())
		stream.owner.operation.RetainCleanup(physical)
		stream.mu.Lock()
		if stream.recoveryLoss == nil {
			stream.recoveryLoss = work
		}
		stream.recoveryCleanup = errors.Join(stream.recoveryCleanup, physical)
		close(records.retired)
		stream.mu.Unlock()
	}()
	// A bound expires independently of a blocked physical join. Deny logical
	// effects on time, but retain the actor until that exact join really returns.
	defer func() { <-records.retired }()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-stream.ctx.Done():
	case <-records.replaced:
	case <-timer.C:
		stream.mu.Lock()
		if stream.records == records && !stream.closing {
			stream.failLocked(recoveryFailure(records.recoveryCause, errRecoveryExhausted))
		}
		stream.mu.Unlock()
	}
}

type originalAttachmentFailure struct{ cause error }

func (failure originalAttachmentFailure) Error() string {
	return "original Service Connection attachment failed"
}
func (failure originalAttachmentFailure) Unwrap() error { return failure.cause }

func recoveryFailure(cause, outcome error) error {
	if cause == nil {
		return outcome
	}
	return errors.Join(outcome, originalAttachmentFailure{cause: cause})
}
