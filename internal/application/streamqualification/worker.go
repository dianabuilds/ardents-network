package streamqualification

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

const scheduleTick = 20 * time.Millisecond

type workerStream struct {
	challenge       [32]byte
	canaryBuffer    []byte
	awaitingCanary  bool
	nextCanary      time.Time
	closed          bool
	id              uint32
	receiveCredit   uint32
	sendCredit      uint32
	offset          uint64
	opened          bool
	terminalPending bool
	receivedEOF     bool
	sentEOF         bool
}

type workerInput struct {
	frame workerFrame
	err   error
}

// RunWorker transfers only predeclared useful bytes over streams already
// opened by Endpoint. It neither creates a Service stream nor receives a
// destination, authority, Route, Principal or Grant.
func RunWorker(ctx context.Context, attachment io.ReadWriteCloser, init Init) (outcome error) {
	if ctx == nil || attachment == nil || init.Nonce == [32]byte{} || init.Seed == [32]byte{} {
		return errors.New("qualification worker is unavailable")
	}
	schedule, err := init.Profile.Definition(init.Role)
	if err != nil {
		return err
	}
	lifetime, cancel := context.WithCancel(ctx)
	defer cancel()
	inputs := make(chan workerInput, 1)
	readDone := make(chan struct{})
	interruptDone := make(chan struct{})
	interrupt := context.AfterFunc(lifetime, func() { defer close(interruptDone); _ = attachment.Close() })
	defer func() {
		cancel()
		if !interrupt() {
			<-interruptDone
		}
		outcome = errors.Join(outcome, attachment.Close())
		<-readDone
	}()
	go func() {
		defer close(readDone)
		for {
			frame, frameErr := readWorkerFrame(attachment)
			select {
			case inputs <- workerInput{frame: frame, err: frameErr}:
			case <-lifetime.Done():
				return
			}
			if frameErr != nil || lifetime.Err() != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(scheduleTick)
	defer ticker.Stop()
	streams := make(map[uint32]*workerStream, schedule.OpenConnections)
	var order []uint32
	var lastID uint32
	var started time.Time
	var pendingEOF []workerFrame
	sender := (init.Role == ReaderRole) == (init.Profile == ClientToPublisher)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case input := <-inputs:
			if errors.Is(input.err, io.EOF) {
				if len(streams) != int(schedule.OpenConnections) {
					return errors.New("qualification attachment ended before full stream set")
				}
				for _, stream := range streams {
					if !stream.closed {
						return errors.New("qualification attachment ended before stream completion")
					}
				}
				return nil
			}
			if input.err != nil {
				return input.err
			}
			if stream := streams[input.frame.id]; stream != nil && stream.terminalPending && input.frame.kind != frameCredit {
				return errors.New("qualification peer sent data after pending EOF")
			}
			if !sender && input.frame.kind == frameEOF && !started.IsZero() && time.Since(started) < 10*time.Minute {
				stream := streams[input.frame.id]
				if stream == nil {
					return errors.New("qualification EOF for absent stream")
				}
				stream.terminalPending = true
				pendingEOF = append(pendingEOF, input.frame)
				continue
			}
			if err := acceptWorkerFrame(attachment, streams, &order, &lastID, init, schedule, sender, input.frame); err != nil {
				return err
			}
			if started.IsZero() && qualificationWorkloadReady(streams, order, schedule) {
				started = time.Now()
			}
		case now := <-ticker.C:
			if !sender && !started.IsZero() && now.Sub(started) >= 10*time.Minute {
				for _, frame := range pendingEOF {
					if err := acceptWorkerFrame(attachment, streams, &order, &lastID, init, schedule, sender, frame); err != nil {
						return err
					}
				}
				pendingEOF = nil
			}
			if !sender || started.IsZero() {
				continue
			}
			if now.Sub(started) >= 10*time.Minute {
				if err := sendScheduledElapsed(attachment, streams, order, init.Seed, schedule, 10*time.Minute); err != nil {
					return err
				}
				for _, id := range order[:int(schedule.ActiveConnections)] {
					if streams[id].offset < offeredWorkloadBits(schedule)*600/8/uint64(schedule.ActiveConnections) {
						return errors.New("qualification workload could not deliver its fixed offered load")
					}
				}
				if err := requireCompletedCanaries(streams, order, schedule); err != nil {
					return err
				}
				if err := closeScheduledStreams(attachment, streams, order, uint16(len(order))); err != nil {
					return err
				}
				continue
			}
			if now.Sub(started) < 598*time.Second {
				if err := sendCanaries(attachment, streams, order, schedule, now); err != nil {
					return err
				}
			}
			if err := sendScheduledElapsed(attachment, streams, order, init.Seed, schedule, now.Sub(started)); err != nil {
				return err
			}
		}
	}
}

func qualificationWorkloadReady(streams map[uint32]*workerStream, order []uint32, schedule Schedule) bool {
	if len(order) != int(schedule.OpenConnections) {
		return false
	}
	for _, id := range order {
		stream := streams[id]
		if stream == nil || !stream.opened || stream.sendCredit == 0 {
			return false
		}
	}
	return true
}

func acceptWorkerFrame(attachment io.Writer, streams map[uint32]*workerStream, order *[]uint32, lastID *uint32, init Init, schedule Schedule, sender bool, frame workerFrame) error {
	if frame.kind == frameOpen {
		if frame.id > 511 || frame.id%2 != 1 || frame.id <= *lastID || len(streams) >= int(schedule.OpenConnections) {
			return errors.New("qualification worker stream opening is invalid")
		}
		streams[frame.id] = &workerStream{id: frame.id, receiveCredit: frameCreditWindow}
		*order, *lastID = append(*order, frame.id), frame.id
		if len(streams) == int(schedule.OpenConnections) {
			active := 0
			for id := range streams {
				if id <= 127 {
					active++
				}
			}
			if active != int(schedule.ActiveConnections) {
				return errors.New("qualification full set has wrong active positions")
			}
		}
		return nil
	}
	stream := streams[frame.id]
	if stream == nil || stream.closed && frame.kind != frameCredit {
		return errors.New("qualification worker stream is unavailable")
	}
	stream.opened = true
	switch frame.kind {
	case frameCredit:
		if binary.BigEndian.Uint32(frame.body) == 0 || frameCreditWindow-stream.sendCredit < binary.BigEndian.Uint32(frame.body) {
			return errors.New("qualification worker credit is invalid")
		}
		stream.sendCredit += binary.BigEndian.Uint32(frame.body)
	case frameBytes:
		if stream.id > 127 {
			return acceptCanaryBytes(attachment, stream, sender, frame.body)
		}
		if sender || stream.receivedEOF || uint32(len(frame.body)) > stream.receiveCredit || !matchesScheduledBytes(init.Seed, stream.id, stream.offset, frame.body) {
			return errors.New("qualification worker received bytes are invalid")
		}
		stream.offset += uint64(len(frame.body))
		stream.receiveCredit -= uint32(len(frame.body))
		var credit [4]byte
		binary.BigEndian.PutUint32(credit[:], uint32(len(frame.body)))
		if err := writeWorkerFrame(attachment, workerFrame{kind: frameCredit, id: stream.id, body: credit[:]}); err != nil {
			return err
		}
		stream.receiveCredit += uint32(len(frame.body))
	case frameEOF:
		if stream.receivedEOF || sender && !stream.sentEOF || len(stream.canaryBuffer) != 0 || stream.awaitingCanary {
			return errors.New("qualification worker EOF is invalid")
		}
		stream.receivedEOF = true
		if !stream.sentEOF {
			if err := writeWorkerFrame(attachment, workerFrame{kind: frameEOF, id: stream.id}); err != nil {
				return err
			}
			stream.sentEOF = true
		}
		if err := writeWorkerFrame(attachment, workerFrame{kind: frameClose, id: stream.id, body: []byte{0}}); err != nil {
			return err
		}
		stream.closed = true
	case frameClose:
		if !stream.sentEOF || !stream.receivedEOF || frame.body[0] != 0 {
			return errors.New("qualification worker stream closed unsuccessfully")
		}
		stream.closed = true
	}
	return nil
}

func closeScheduledStreams(attachment io.Writer, streams map[uint32]*workerStream, order []uint32, active uint16) error {
	for index, id := range order {
		if index >= int(active) {
			break
		}
		stream := streams[id]
		if stream == nil || stream.sentEOF {
			continue
		}
		if err := writeWorkerFrame(attachment, workerFrame{kind: frameEOF, id: id}); err != nil {
			return err
		}
		stream.sentEOF = true
	}
	return nil
}

func scheduledBytes(seed [32]byte, id uint32, offset uint64, size int) []byte {
	body := make([]byte, size)
	for written := 0; written < len(body); {
		var input [44]byte
		copy(input[:32], seed[:])
		binary.BigEndian.PutUint32(input[32:36], id)
		binary.BigEndian.PutUint64(input[36:44], offset/sha256.Size)
		digest := sha256.Sum256(input[:])
		start := int(offset % sha256.Size)
		copied := copy(body[written:], digest[start:])
		written += copied
		offset += uint64(copied)
	}
	return body
}
func matchesScheduledBytes(seed [32]byte, id uint32, offset uint64, body []byte) bool {
	if len(body) == 0 || len(body) > frameLimit {
		return false
	}
	expected := scheduledBytes(seed, id, offset, len(body))
	for index := range body {
		if body[index] != expected[index] {
			return false
		}
	}
	return true
}
