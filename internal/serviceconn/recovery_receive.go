package serviceconn

import (
	"bytes"
	"errors"
	"io"
	"sort"
)

func (stream *recoveryStream) receiveApplication(receiveLimit, sendLimit uint64) error {
	for {
		stream.mu.Lock()
		complete := stream.recvNext == receiveLimit && stream.sendBase == sendLimit
		terminal := stream.terminal
		stream.mu.Unlock()
		if terminal != nil {
			return terminal
		}
		if complete {
			return nil
		}
		attachment, err := stream.attachment()
		if err != nil {
			return err
		}
		frame, err := readConnectionFrame(attachment.connection)
		if err != nil {
			stream.mu.Lock()
			complete = stream.recvNext == receiveLimit && stream.sendBase == sendLimit
			stream.mu.Unlock()
			if complete && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe)) {
				return nil
			}
			if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
				return errors.Join(errRecoveryTerminal, err, recoverErr)
			}
			continue
		}
		if frame.generation != attachment.generation {
			return errActiveViolation
		}
		switch frame.kind {
		case ackFrameType:
			stream.mu.Lock()
			err = stream.acknowledgeLocked(frame.offset)
			stream.mu.Unlock()
		case dataFrameType:
			err = stream.acceptData(attachment, frame, receiveLimit)
		case terminalFrameType:
			stream.mu.Lock()
			valid := frame.offset == stream.recvNext
			stream.mu.Unlock()
			if !valid {
				return errActiveViolation
			}
			return errors.New("remote Application stream ended before the declared byte count")
		}
		if err != nil {
			return err
		}
	}
}

func (stream *recoveryStream) acknowledgeLocked(offset uint64) error {
	if offset < stream.sendBase {
		return nil
	}
	if offset > stream.sendEnd {
		return errActiveViolation
	}
	trim := offset - stream.sendBase
	stream.sendData = append(stream.sendData[:0], stream.sendData[trim:]...)
	stream.sendBase = offset
	if stream.sendNext < offset {
		stream.sendNext = offset
	}
	stream.cond.Broadcast()
	return nil
}

func (stream *recoveryStream) acceptData(attachment *securedAttachment, frame connectionFrame, limit uint64) error {
	end := frame.offset + uint64(len(frame.data))
	if len(frame.data) == 0 || end < frame.offset || end > limit {
		return errActiveViolation
	}
	stream.mu.Lock()
	next := stream.recvNext
	if frame.offset < next {
		prefix := next - frame.offset
		if prefix > uint64(len(frame.data)) {
			prefix = uint64(len(frame.data))
		}
		if !stream.matchesRecentLocked(frame.offset, frame.data[:prefix]) {
			stream.mu.Unlock()
			return errActiveViolation
		}
		frame.offset += prefix
		frame.data = frame.data[prefix:]
		if len(frame.data) == 0 {
			stream.mu.Unlock()
			stream.queueAcknowledgement(next)
			return nil
		}
	}
	if err := stream.storeRangeLocked(receivedRange{offset: frame.offset,
		data: append([]byte(nil), frame.data...)}); err != nil {
		stream.mu.Unlock()
		return err
	}
	for len(stream.pending) > 0 && stream.pending[0].offset == stream.recvNext {
		ready := stream.pending[0]
		stream.pending = stream.pending[1:]
		stream.mu.Unlock()
		if err := writeAll(stream.application, ready.data); err != nil {
			return err
		}
		stream.mu.Lock()
		stream.recent = append(stream.recent, ready.data...)
		stream.recvNext += uint64(len(ready.data))
		stream.trimRecentLocked(stream.pendingBytesLocked())
	}
	next = stream.recvNext
	stream.mu.Unlock()
	stream.queueAcknowledgement(next)
	return nil
}

func (stream *recoveryStream) storeRangeLocked(candidate receivedRange) error {
	retained := stream.pending[:0]
	for _, existing := range stream.pending {
		candidateEnd := candidate.offset + uint64(len(candidate.data))
		existingEnd := existing.offset + uint64(len(existing.data))
		if candidateEnd < existing.offset || existingEnd < candidate.offset {
			retained = append(retained, existing)
			continue
		}
		overlapStart := max(candidate.offset, existing.offset)
		overlapEnd := min(candidateEnd, existingEnd)
		if overlapStart < overlapEnd && !bytes.Equal(
			candidate.data[overlapStart-candidate.offset:overlapEnd-candidate.offset],
			existing.data[overlapStart-existing.offset:overlapEnd-existing.offset]) {
			return errActiveViolation
		}
		mergedStart, mergedEnd := min(candidate.offset, existing.offset), max(candidateEnd, existingEnd)
		merged := make([]byte, mergedEnd-mergedStart)
		copy(merged[existing.offset-mergedStart:], existing.data)
		copy(merged[candidate.offset-mergedStart:], candidate.data)
		candidate = receivedRange{offset: mergedStart, data: merged}
	}
	retained = append(retained, candidate)
	sort.Slice(retained, func(first, second int) bool { return retained[first].offset < retained[second].offset })
	if len(retained) > 8 {
		return errActiveViolation
	}
	stream.pending = retained
	pending := stream.pendingBytesLocked()
	stream.trimRecentLocked(pending)
	if pending+len(stream.recent) > logicalQueueLimit {
		return errActiveViolation
	}
	if queued := uint32(pending + len(stream.recent)); queued > stream.queueMax {
		stream.queueMax = queued
	}
	return nil
}

func (stream *recoveryStream) pendingBytesLocked() int {
	total := 0
	for _, value := range stream.pending {
		total += len(value.data)
	}
	return total
}

func (stream *recoveryStream) trimRecentLocked(reserved int) {
	keep := logicalQueueLimit - reserved
	if keep < 0 {
		keep = 0
	}
	if len(stream.recent) > keep {
		trim := len(stream.recent) - keep
		stream.recent = append(stream.recent[:0], stream.recent[trim:]...)
		stream.recentAt += uint64(trim)
	}
}

func (stream *recoveryStream) matchesRecentLocked(offset uint64, data []byte) bool {
	end := offset + uint64(len(data))
	if end < offset || offset < stream.recentAt || end > stream.recentAt+uint64(len(stream.recent)) {
		return false
	}
	start := offset - stream.recentAt
	return bytes.Equal(stream.recent[start:start+uint64(len(data))], data)
}

func (stream *recoveryStream) queueAcknowledgement(offset uint64) {
	stream.mu.Lock()
	if offset > stream.ackPending {
		stream.ackPending = offset
	}
	stream.mu.Unlock()
	select {
	case stream.ackSignal <- struct{}{}:
	default:
	}
}
