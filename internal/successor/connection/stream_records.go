package connection

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
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
	for {
		physicalRead := observedRecordRead{reader: records.physical}
		decoded, readErr := wire.ReadStream(&physicalRead)
		record := wire.Record{Data: decoded.Data, Acknowledgement: decoded.Acknowledgement, Terminal: decoded.Terminal}
		result = readErr
		// The decoded clean CLOSE is an immutable fact of this exact child,
		// not logical authority. Observe it outside the consumer state lock;
		// commit still matches the same records and complete Terminal state.
		peerCloseErr := errIntegrity
		if terminalTruncation(readErr, physicalRead.received) {
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
				if stream.records != records || stream.receive.generation != records.generation {
					return errIntegrity
				}
				if readErr != nil {
					if stream.terminalTailCompleteLocked() && terminalTruncation(readErr, physicalRead.received) && peerCloseErr == nil {
						finished, retireTail = true, true
						return nil
					}
					if !errors.Is(readErr, wire.ErrInvalidRecord) && (stream.publisherRoute != nil || stream.initiatorRoute != nil) {
						if err := stream.startRecoveryLocked(records); err != nil {
							return errors.Join(readErr, err)
						}
						finished = true
						return nil
					}
					return readErr
				}
				before := stream.recordProgressLocked()
				if err := stream.acceptRecordLocked(records, record); err != nil {
					if errors.Is(err, errBackpressure) {
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
				stream.cancel()
				stream.mu.Lock()
				stream.signalLocked()
				stream.mu.Unlock()
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

func (stream *Stream) acceptRecordLocked(records *attachmentRecords, record wire.Record) error {
	switch {
	case record.Data != nil:
		if stream.settled {
			return errIntegrity
		}
		if err := stream.receive.accept(*record.Data); err != nil {
			return err
		}
		if stream.ackSerial == ^uint64(0) {
			return errIntegrity
		}
		stream.ackSerial++
	case record.Terminal != nil:
		if err := stream.receive.closeInput(*record.Terminal); err != nil {
			return err
		}
		if _, err := stream.terminal.receiveTerminal(*record.Terminal, stream.receive.next, false); err != nil {
			return err
		}
	case record.Acknowledgement != nil:
		value := *record.Acknowledgement
		if value.Terminal {
			if err := stream.terminal.receiveAcknowledgement(value); err != nil {
				return err
			}
			if value.TerminalConfirmation {
				return nil
			}
		}
		return stream.send.acknowledge(value.AttachmentGeneration, value.Offset)
	default:
		// Initial/recovery authentication cannot arrive on an active stream.
		return errIntegrity
	}
	return nil
}

// One serialized writer sends accepted Data before its Terminal. ACK serials
// retain acknowledgements queued while another physical write is in flight.
func (stream *Stream) writeRecords(records *attachmentRecords) {
	var result error
	defer func() { records.writerResult = result; close(records.writerDone) }()
	for {
		var record wire.Record
		var data *dataWrite
		var control *terminalWrite
		var ackSerial uint64
		var changed <-chan struct{}
		finished := false
		err := stream.commit(func() error {
			if records.retiring {
				finished = true
				return nil
			}
			if stream.records != records || stream.send.generation != records.generation {
				return errIntegrity
			}
			var err error
			switch {
			case stream.ackWritten != stream.ackSerial:
				ackSerial = stream.ackSerial
				record.Acknowledgement = &wire.Acknowledgement{AttachmentGeneration: stream.receive.generation, Offset: stream.receive.next}
			case stream.send.next != stream.send.end:
				data, err = stream.send.beginWrite()
				if data != nil {
					record.Data = &data.record
				}
			default:
				control, err = stream.terminal.beginWrite(stream.send.next)
				if control != nil {
					record.Terminal = control.record.Terminal
					record.Acknowledgement = control.record.Acknowledgement
				}
			}
			if record.Data == nil && record.Acknowledgement == nil && record.Terminal == nil && err == nil {
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
			if stream.records != records || stream.send.generation != records.generation {
				return errors.Join(errIntegrity, writeErr)
			}
			before := stream.recordProgressLocked()
			err := writeErr
			if data != nil {
				_, err = stream.send.finishWrite(data, err)
			} else if control != nil {
				_, err = stream.terminal.finishWrite(control, err)
			} else if err == nil {
				stream.ackWritten = ackSerial
			}
			if records.retiring {
				finished = true
				return nil
			}
			if err != nil {
				if !errors.Is(err, wire.ErrInvalidRecord) && (stream.publisherRoute != nil || stream.initiatorRoute != nil) {
					if recoveryErr := stream.startRecoveryLocked(records); recoveryErr != nil {
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
type recordProgress struct {
	sendBase, sentNext, receivedNext                                       uint64
	remoteClosed, remoteTerminalCurrent, receiptReceived, receiptConfirmed bool
	terminalWritten, receiptWritten, confirmationWritten                   bool
}

func (stream *Stream) recordProgressLocked() recordProgress {
	return recordProgress{stream.send.base, stream.send.next, stream.receive.next,
		stream.terminal.remoteClosed, stream.terminal.remoteTerminalCurrent, stream.terminal.receiptReceived, stream.terminal.receiptConfirmed,
		stream.terminal.terminalWritten, stream.terminal.receiptWritten, stream.terminal.confirmationWritten}
}

func (stream *Stream) noteRecordProgressLocked(before recordProgress) {
	if before == stream.recordProgressLocked() || stream.checkLocked() != nil {
		return
	}
	at := time.Now()
	stream.lastProgress = at
	if stream.recovery != nil {
		stream.recovery.progress(at)
	}
}
