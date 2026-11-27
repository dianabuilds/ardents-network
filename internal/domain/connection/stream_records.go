package connection

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/recordio"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// attachmentRecords retains the two exact physical record actors together.
// A later Attachment needs its own actors and completion channels; waiting on
// this owner cannot accidentally include a replacement or omit the old writer.
// Only OpenStream constructs the initial owner from sealed authentication.
type attachmentRecords struct {
	generation                 uint64
	physical                   attachmentIO
	readerDone, writerDone     chan struct{}
	retiring                   bool // Stream.mu seals both actors before interruption.
	retired, replaced          chan struct{}
	recoveryCause              error // First trigger, assigned under Stream.mu.
	readerResult, writerResult error // Each actor publishes before its done channel.
}

func (records *attachmentRecords) join() error {
	if records != nil {
		<-records.readerDone
		<-records.writerDone
		return errors.Join(records.readerResult, records.writerResult)
	}
	return nil
}

// One parser retains the current Attachment. Application backpressure cannot
// move its record to another generation or discard unread logical bytes.
func (stream *Stream) readRecords(records *attachmentRecords) {
	var result error
	defer func() { records.readerResult = result; close(records.readerDone) }()
	input := recordio.NewStreamReader(records.physical)
	defer input.Discard()
	for {
		decoded, received, readErr := input.ReadRecord()
		record := wire.Record{Data: decoded.Data, Acknowledgement: decoded.Acknowledgement, Terminal: decoded.Terminal}
		result = readErr
		// The decoded clean CLOSE is an immutable fact of this exact child,
		// not logical authority. Observe it outside the consumer state lock;
		// commit still matches the same records and complete Terminal state.
		peerCloseErr := errIntegrity
		if terminalTruncation(readErr, received) {
			peerCloseErr = records.physical.decodedPeerClose()
		}
		for {
			var changed <-chan struct{}
			finished, retireTail := false, false
			err := stream.commit(func() error {
				if records.retiring {
					finished = true
					return nil
				}
				if stream.records != records || stream.transfer.Generation() != records.generation {
					return errIntegrity
				}
				if readErr != nil {
					if stream.terminalTailCompleteLocked() && terminalTruncation(readErr, received) && peerCloseErr == nil {
						finished, retireTail = true, true
						return nil
					}
					if !errors.Is(readErr, wire.ErrInvalidRecord) && (stream.publisherRoute != nil || stream.initiatorRoute != nil) {
						if err := stream.startRecoveryLocked(records, readErr); err != nil {
							return errors.Join(readErr, err)
						}
						finished = true
						return nil
					}
					return readErr
				}
				before := stream.transfer.Progress()
				if err := stream.acceptRecordLocked(record); err != nil {
					if errors.Is(err, transfer.ErrBackpressure) {
						changed = stream.changeLocked()
						return nil
					}
					return err
				}
				stream.noteRecordProgressLocked(before)
				stream.signalLocked()
				return nil
			})
			if err != nil {
				result = errors.Join(readErr, err)
				stream.failRecord(records, result)
				return
			}
			if retireTail {
				result = nil // Exact decoded clean CLOSE after authenticated Terminal.
				stream.sealRetirement()
			}
			if finished {
				return
			}
			if changed == nil {
				break
			}
			stream.waitChange(changed)
		}
	}
}

func (stream *Stream) failRecord(records *attachmentRecords, cause error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closing || records.retiring {
		return
	}
	// Cancellation already sealed this logical lifetime. Its watcher may be
	// waiting for this mutex while physical interruption returns a read error.
	// Keep the earlier cause; exact actors and lower cleanup retain their errors.
	if stream.ctx.Err() != nil {
		if stream.failure == nil {
			stream.failure = context.Cause(stream.ctx)
		}
		return
	}
	if stream.records != records {
		cause = errors.Join(errIntegrity, cause)
	}
	stream.failLocked(cause)
}

func (stream *Stream) acceptRecordLocked(record wire.Record) error {
	if stream.settled && record.Data != nil {
		return errIntegrity
	}
	return stream.transfer.AcceptRecord(record)
}

// One serialized writer sends accepted Data before its Terminal. ACK serials
// retain acknowledgements queued while another physical write is in flight.
func (stream *Stream) writeRecords(records *attachmentRecords) {
	var result error
	defer func() { records.writerResult = result; close(records.writerDone) }()
	for {
		var record wire.Record
		var ticket *transfer.RecordWrite
		var changed <-chan struct{}
		finished := false
		err := stream.commit(func() error {
			if records.retiring {
				finished = true
				return nil
			}
			if stream.records != records || stream.transfer.Generation() != records.generation {
				return errIntegrity
			}
			var err error
			record, ticket, err = stream.transfer.BeginRecordWrite()
			if ticket == nil && err == nil {
				changed = stream.changeLocked()
			}
			return err
		})
		if err != nil {
			result = err
			stream.failRecord(records, err)
			return
		}
		if finished {
			return
		}
		if changed != nil {
			stream.waitChange(changed)
			continue
		}
		writeErr := wire.Write(records.physical, record)
		result = writeErr
		err = stream.commit(func() error {
			if stream.records != records || stream.transfer.Generation() != records.generation {
				return errors.Join(errIntegrity, writeErr)
			}
			before := stream.transfer.Progress()
			err := stream.transfer.FinishRecordWrite(ticket, writeErr)
			if records.retiring {
				finished = true
				return nil
			}
			if err != nil {
				if !errors.Is(err, wire.ErrInvalidRecord) && (stream.publisherRoute != nil || stream.initiatorRoute != nil) {
					if recoveryErr := stream.startRecoveryLocked(records, err); recoveryErr != nil {
						return errors.Join(err, recoveryErr)
					}
					finished = true
					return nil
				}
				return err
			}
			stream.noteRecordProgressLocked(before)
			stream.signalLocked()
			return nil
		})
		if err != nil {
			result = errors.Join(writeErr, err)
			stream.failRecord(records, result)
			return
		}
		if finished {
			return
		}
	}
}

// Matching replay and repeated ACKs cannot renew the progress anchor. Only
// changed authenticated byte/control facts do; an active episode stays fixed.
func (stream *Stream) noteRecordProgressLocked(before transfer.Progress) {
	if before == stream.transfer.Progress() || stream.checkLocked() != nil {
		return
	}
	at := time.Now()
	stream.lastProgress = at
	if stream.recovery != nil {
		stream.recovery.progress(at)
	}
}
