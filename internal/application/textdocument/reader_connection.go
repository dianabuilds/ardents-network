//go:build linux

package textdocument

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

// ReadWorkerConnection forwards one already-authorized Service Connection to
// an initialized reader worker. Endpoint retains destination/Grant ownership.
// The returned bytes have passed local RESULT checks and joined stream I/O;
// Endpoint must additionally join the worker cgroup and check its current job
// before presenting them. No failure opens another Connection or retries.
func ReadWorkerConnection(ctx context.Context, attachment io.ReadWriteCloser, service connection.Stream) (body []byte, resultErr error) {
	if ctx == nil || attachment == nil || service == nil {
		return nil, errors.New("text reader Connection is unavailable")
	}
	owner := startReaderConnectionIO(ctx, attachment, service)
	state := readerConnectionState{io: owner, sendCredit: workerCredit, receiveCredit: workerCredit, resultCredit: workerCredit}
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(callbackDone); _ = owner.close() })
	defer func() {
		resultErr = errors.Join(resultErr, owner.close(), ctx.Err())
		if !stop() {
			<-callbackDone
		}
		if resultErr != nil {
			clear(state.result)
			body = nil
		}
	}()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := writeWorkerFrame(attachment, workerFrame{kind: 1, id: 1}); err != nil {
		return nil, errors.New("text reader stream could not open")
	}
	for {
		if state.serviceEOF && state.remoteClean && state.requestEOF && state.requestDrained && !state.closeSent {
			if err := writeWorkerFrame(attachment, workerFrame{kind: 5, id: 1, body: []byte{0}}); err != nil {
				return nil, errors.New("text reader terminal was interrupted")
			}
			state.closeSent = true
		}
		var permit chan int
		if !state.readOutstanding && !state.serviceEOF && state.sendCredit > 0 {
			permit = owner.permits
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case permit <- min(8<<10, int(state.sendCredit)):
			state.readOutstanding = true
		case event := <-owner.serviceEvents:
			if err := state.fromService(event); err != nil {
				return nil, err
			}
		case completed := <-owner.written:
			if completed.err != nil {
				return nil, errors.New("text reader request could not reach Service")
			}
			if completed.eof {
				state.requestDrained = true
			} else {
				state.receiveCredit += completed.size
				if err := state.credit(1, completed.size); err != nil {
					return nil, err
				}
			}
		case event := <-owner.frames:
			if event.err != nil {
				return nil, errors.New("text reader worker exchange was interrupted")
			}
			complete, err := state.fromWorker(event.frame)
			if err != nil {
				return nil, err
			}
			if complete {
				if err := writeWorkerFrame(attachment, workerFrame{kind: 5, id: 2, body: []byte{0}}); err != nil {
					return nil, errors.New("text reader result terminal was interrupted")
				}
				if state.status != 0 {
					return nil, errors.New("text document is unavailable")
				}
				return state.result, nil
			}
		}
	}
}

type readerConnectionState struct {
	request         [requestBytes]byte
	io              *readerConnectionIO
	sendCredit      uint32
	receiveCredit   uint32
	resultCredit    uint32
	serviceBytes    uint32
	requestBytes    uint32
	readOutstanding bool
	serviceEOF      bool
	remoteClean     bool
	requestEOF      bool
	requestDrained  bool
	closeSent       bool
	resultAnnounced bool
	status          byte
	resultLength    uint32
	result          []byte
}

func (state *readerConnectionState) fromService(event readerServiceEvent) error {
	if event.err != nil {
		return event.err
	}
	if len(event.body) > 0 {
		size := uint32(len(event.body))
		if state.serviceEOF || size > state.sendCredit || size > MaximumBytes+13-state.serviceBytes {
			return errors.New("text Service response exceeds its bound")
		}
		state.readOutstanding = false
		state.sendCredit -= size
		state.serviceBytes += size
		if err := writeWorkerFrame(state.io.attachment, workerFrame{kind: 2, id: 1, body: event.body}); err != nil {
			return errors.New("text Service response forwarding was interrupted")
		}
	}
	if event.eof {
		if state.serviceEOF {
			return errors.New("text Service repeated EOF")
		}
		state.serviceEOF = true
		if err := writeWorkerFrame(state.io.attachment, workerFrame{kind: 4, id: 1}); err != nil {
			return errors.New("text Service EOF forwarding was interrupted")
		}
	}
	if event.terminal {
		if !state.serviceEOF || state.remoteClean || event.outcome.Class != connection.CleanClose {
			return errors.New("text Service Connection did not complete")
		}
		state.remoteClean = true
	}
	return nil
}

func (state *readerConnectionState) fromWorker(frame workerFrame) (bool, error) {
	if frame.id == 2 {
		return state.fromResult(frame)
	}
	if frame.id != 1 || state.closeSent && (frame.kind != 3 || state.resultAnnounced) {
		return false, errors.New("text worker emitted an unsolicited stream")
	}
	switch frame.kind {
	case 2:
		size := uint32(len(frame.body))
		if state.requestEOF || size > state.receiveCredit || size > requestBytes-state.requestBytes {
			return false, errors.New("text worker request exceeds its bound")
		}
		state.receiveCredit -= size
		copy(state.request[state.requestBytes:], frame.body)
		state.requestBytes += size
	case 3:
		credit := binary.BigEndian.Uint32(frame.body)
		if credit == 0 || credit > workerCredit || state.sendCredit > workerCredit-credit {
			return false, errors.New("text worker response credit is invalid")
		}
		state.sendCredit += credit
	case 4:
		if state.requestEOF || state.requestBytes != requestBytes || !validDocumentRequest(state.request[:]) {
			return false, errors.New("text worker request EOF is invalid")
		}
		state.requestEOF = true
		// The fixed request fits in 512 bytes. Queue it only after exact EOF;
		// arbitrarily fragmented BYTES cannot fill mutually dependent I/O queues.
		select {
		case state.io.writes <- workerFrame{kind: 2, id: 1, body: state.request[:]}:
		case <-state.io.ctx.Done():
			return false, state.io.ctx.Err()
		}
		select {
		case state.io.writes <- frame:
		case <-state.io.ctx.Done():
			return false, state.io.ctx.Err()
		}
	default:
		return false, errors.New("text worker reader stream is invalid")
	}
	return false, nil
}

func (state *readerConnectionState) fromResult(frame workerFrame) (bool, error) {
	if !state.closeSent {
		return false, errors.New("text worker result preceded authenticated completion")
	}
	switch frame.kind {
	case 6:
		if state.resultAnnounced {
			return false, errors.New("text worker repeated its result")
		}
		state.status = frame.body[0]
		state.resultLength = binary.BigEndian.Uint32(frame.body[1:])
		if state.status > 1 || state.resultLength > MaximumBytes || state.status == 1 && state.resultLength != 0 {
			return false, errors.New("text worker result is invalid")
		}
		state.resultAnnounced = true
		state.result = make([]byte, 0, int(state.resultLength))
	case 2:
		size := uint32(len(frame.body))
		if !state.resultAnnounced || size > state.resultCredit || size > state.resultLength-uint32(len(state.result)) {
			return false, errors.New("text worker result exceeds its bound")
		}
		state.resultCredit -= size
		state.result = append(state.result, frame.body...)
		// The bounded final result has consumed these bytes. No unbounded queue or
		// further network request can be created by replenishing this local credit.
		if err := state.credit(2, size); err != nil {
			return false, err
		}
		state.resultCredit += size
	case 4:
		if !state.resultAnnounced || uint32(len(state.result)) != state.resultLength || !utf8.Valid(state.result) {
			return false, errors.New("text worker result is incomplete or invalid")
		}
		return true, nil
	default:
		return false, errors.New("text worker result frame is invalid")
	}
	return false, nil
}

func (state *readerConnectionState) credit(id, size uint32) error {
	if size == 0 {
		return errors.New("text worker credit cannot be empty")
	}
	var body [4]byte
	binary.BigEndian.PutUint32(body[:], size)
	if err := writeWorkerFrame(state.io.attachment, workerFrame{kind: 3, id: id, body: body[:]}); err != nil {
		return errors.New("text worker credit forwarding was interrupted")
	}
	return nil
}
