//go:build linux

package textdocument

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

const workerCredit = 64 << 10

type workerStream struct {
	closed        atomic.Bool
	rejected      bool
	closeSent     bool
	id            uint32
	input         []byte
	receiveCredit uint32
	sendCredit    uint32
	receivedEOF   bool
	sentEOF       bool
	output        []byte
	header        []byte
	offset        int
	ready         bool
}

type workerMultiplexer struct {
	initial         workerInitialization
	streams         map[uint32]*workerStream
	order           []uint32
	lastID          uint32
	cursor          int
	readerResult    bool
	resultAnnounced bool
}

// RunWorker implements only the fixed, already launched text worker exchange.
// It cannot select a destination or open a network/file resource. Its caller
// must verify the inherited attachment; readiness never grants local authority.
func RunWorker(ctx context.Context, attachment io.ReadWriteCloser, mode WorkerMode) (resultErr error) {
	if ctx == nil || attachment == nil {
		return errors.New("text worker attachment is unavailable")
	}
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(lifetime, func() { _ = attachment.Close() })
	defer stop()
	defer func() { resultErr = errors.Join(resultErr, attachment.Close()) }()
	initial, err := readWorkerInitialization(attachment, mode)
	if err != nil {
		return err
	}
	defer clear(initial.snapshot)
	if err := writeWorkerReadiness(attachment, initial); err != nil {
		return err
	}
	owner := workerMultiplexer{initial: initial, streams: make(map[uint32]*workerStream)}
	frames := make(chan workerFrame)
	failures := make(chan error, 2)
	outgoing := make(chan workerFrame, 64)
	var work sync.WaitGroup
	work.Add(2)
	go func() {
		defer work.Done()
		for {
			frame, err := readWorkerFrame(attachment)
			if err != nil {
				failures <- err
				return
			}
			select {
			case frames <- frame:
			case <-lifetime.Done():
				return
			}
		}
	}()
	go func() {
		defer work.Done()
		for {
			select {
			case frame := <-outgoing:
				if frame.current != nil && frame.current.closed.Load() {
					continue
				}
				if err := writeWorkerFrame(attachment, frame); err != nil {
					failures <- err
					return
				}
			case <-lifetime.Done():
				return
			}
		}
	}()
	defer func() { cancel(); _ = attachment.Close(); work.Wait() }()
	for {
		next, available := owner.nextFrame()
		var output chan workerFrame
		if available {
			output = outgoing
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case frame := <-frames:
			complete, err := owner.receive(frame)
			if err != nil || complete {
				return err
			}
		case output <- next:
			owner.sent(next)
		}
	}
}

func (owner *workerMultiplexer) receive(frame workerFrame) (bool, error) {
	if frame.kind == 1 {
		if frame.id%2 != 1 || frame.id <= owner.lastID || len(owner.streams) >= 256 || (owner.initial.mode == ReaderWorker && frame.id != 1) {
			return false, errors.New("text worker stream admission is invalid")
		}
		stream := &workerStream{id: frame.id, receiveCredit: workerCredit, sendCredit: workerCredit}
		if owner.initial.mode == ReaderWorker {
			stream.output = make([]byte, requestBytes)
			copy(stream.output, "ARDTXT01\x01")
			stream.ready = true
		}
		owner.streams[frame.id] = stream
		owner.order = append(owner.order, frame.id)
		owner.lastID = frame.id
		return false, nil
	}
	stream := owner.streams[frame.id]
	if stream == nil {
		return false, errors.New("text worker stream is unsolicited")
	}
	switch frame.kind {
	case 2:
		if frame.id == 2 || stream.receivedEOF || uint32(len(frame.body)) > stream.receiveCredit {
			return false, errors.New("text worker input credit is invalid")
		}
		stream.receiveCredit -= uint32(len(frame.body))
		if stream.rejected {
			return false, nil
		}
		maximum := requestBytes
		if owner.initial.mode == ReaderWorker {
			maximum = MaximumBytes + 13
		}
		if len(stream.input)+len(frame.body) > maximum {
			if owner.initial.mode == PublisherWorker {
				stream.rejected = true
				stream.input = nil
				return false, nil
			}
			return false, errors.New("text worker input exceeds its bound")
		}
		stream.input = append(stream.input, frame.body...)
	case 3:
		credit := binary.BigEndian.Uint32(frame.body)
		if credit == 0 || credit > workerCredit || stream.sendCredit > workerCredit-credit {
			return false, errors.New("text worker output credit is invalid")
		}
		stream.sendCredit += credit
	case 4:
		if frame.id == 2 || stream.receivedEOF {
			return false, errors.New("text worker EOF is invalid")
		}
		stream.receivedEOF = true
		if owner.initial.mode == PublisherWorker {
			if stream.rejected || !validDocumentRequest(stream.input) {
				stream.rejected = true
				stream.input = nil
				return false, nil
			}
			header := documentResponseHeader(len(owner.initial.snapshot))
			stream.header = header[:]
			stream.output = owner.initial.snapshot
			stream.input = nil
			stream.ready = true
		}
	case 5:
		if frame.body[0] > 6 {
			return false, errors.New("text worker terminal class is invalid")
		}
		if owner.initial.mode == ReaderWorker && frame.id == 1 {
			if frame.body[0] != 0 || !stream.receivedEOF || !stream.sentEOF || owner.readerResult {
				return false, errors.New("text reader Connection is interrupted")
			}
			if err := owner.prepareResult(stream.input); err != nil {
				return false, err
			}
			owner.readerResult = true
		} else if owner.initial.mode == ReaderWorker && frame.id == 2 {
			if frame.body[0] != 0 || !stream.sentEOF {
				return false, errors.New("text reader result is interrupted")
			}
			return true, nil
		}
		stream.closed.Store(true)
		delete(owner.streams, frame.id)
		for index, id := range owner.order {
			if id == frame.id {
				owner.order = append(owner.order[:index], owner.order[index+1:]...)
				break
			}
		}
	}
	return false, nil
}

func (owner *workerMultiplexer) prepareResult(response []byte) error {
	if len(response) < 13 || string(response[:8]) != magic {
		return errors.New("text reader response is invalid")
	}
	length := binary.BigEndian.Uint32(response[9:13])
	if response[8] > 1 || length > MaximumBytes || len(response) != 13+int(length) || (response[8] == 1 && length != 0) || !utf8.Valid(response[13:]) {
		return errors.New("text reader response is invalid")
	}
	body := response[13:]
	header := make([]byte, 5)
	header[0] = response[8]
	binary.BigEndian.PutUint32(header[1:], length)
	owner.streams[2] = &workerStream{id: 2, header: header, output: body, ready: true, sendCredit: workerCredit}
	owner.order = append(owner.order, 2)
	return nil
}
