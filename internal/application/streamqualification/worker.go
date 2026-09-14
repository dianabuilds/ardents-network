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
	id            uint32
	receiveCredit uint32
	sendCredit    uint32
	offset        uint64
	opened        bool
	receivedEOF   bool
	sentEOF       bool
}

type workerInput struct {
	frame workerFrame
	err   error
}

// RunWorker transfers only predeclared useful bytes over streams already
// opened by Endpoint. It neither creates a Service stream nor receives a
// destination, authority, Route, Principal or Grant.
func RunWorker(ctx context.Context, attachment io.ReadWriter, init Init) error {
	if ctx == nil || attachment == nil || init.Nonce == [32]byte{} || init.Seed == [32]byte{} {
		return errors.New("qualification worker is unavailable")
	}
	schedule, err := init.Profile.Definition(init.Role)
	if err != nil {
		return err
	}
	inputs := make(chan workerInput, 1)
	go func() {
		for {
			frame, frameErr := readWorkerFrame(attachment)
			select {
			case inputs <- workerInput{frame: frame, err: frameErr}:
			case <-ctx.Done():
			}
			if frameErr != nil {
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
	var scheduleCursor int
	sender := (init.Role == ReaderRole) == (init.Profile == ClientToPublisher)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case input := <-inputs:
			if errors.Is(input.err, io.EOF) {
				return nil
			}
			if input.err != nil {
				return input.err
			}
			if err := acceptWorkerFrame(attachment, streams, &order, &lastID, init, schedule, sender, input.frame); err != nil {
				return err
			}
			if started.IsZero() && len(order) >= int(schedule.ActiveConnections) {
				started = time.Now()
			}
		case now := <-ticker.C:
			if !sender || started.IsZero() {
				continue
			}
			if now.Sub(started) >= 10*time.Minute {
				if err := closeScheduledStreams(attachment, streams, order, schedule.ActiveConnections); err != nil {
					return err
				}
				continue
			}
			if err := sendScheduledBytes(attachment, streams, order, &scheduleCursor, init.Seed, schedule); err != nil {
				return err
			}
		}
	}
}

func acceptWorkerFrame(attachment io.Writer, streams map[uint32]*workerStream, order *[]uint32, lastID *uint32, init Init, schedule Schedule, sender bool, frame workerFrame) error {
	if frame.kind == frameOpen {
		if frame.id%2 != 1 || frame.id <= *lastID || len(streams) >= int(schedule.OpenConnections) {
			return errors.New("qualification worker stream opening is invalid")
		}
		streams[frame.id] = &workerStream{id: frame.id, receiveCredit: frameCreditWindow}
		*order, *lastID = append(*order, frame.id), frame.id
		return nil
	}
	stream := streams[frame.id]
	if stream == nil {
		return errors.New("qualification worker stream is unavailable")
	}
	stream.opened = true
	switch frame.kind {
	case frameCredit:
		if !sender || stream.sentEOF || binary.BigEndian.Uint32(frame.body) == 0 || frameCreditWindow-stream.sendCredit < binary.BigEndian.Uint32(frame.body) {
			return errors.New("qualification worker credit is invalid")
		}
		stream.sendCredit += binary.BigEndian.Uint32(frame.body)
	case frameBytes:
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
		if sender || stream.receivedEOF {
			return errors.New("qualification worker EOF is invalid")
		}
		stream.receivedEOF = true
	case frameClose:
		if frame.body[0] != 0 {
			return errors.New("qualification worker stream closed unsuccessfully")
		}
		delete(streams, frame.id)
	}
	return nil
}

func sendScheduledBytes(attachment io.Writer, streams map[uint32]*workerStream, order []uint32, cursor *int, seed [32]byte, schedule Schedule) error {
	active := min(int(schedule.ActiveConnections), len(order))
	if cursor == nil || active == 0 {
		return errors.New("qualification worker schedule is unavailable")
	}
	if *cursor < 0 || *cursor >= active {
		*cursor = 0
	}
	budget := int(schedule.AggregateBits / 8 / uint32(time.Second/scheduleTick))
	for checked := 0; checked < active && budget > 0; checked++ {
		id := order[*cursor]
		*cursor = (*cursor + 1) % active
		stream := streams[id]
		if stream == nil || stream.sentEOF || stream.sendCredit == 0 {
			continue
		}
		size := min(frameLimit, budget, int(stream.sendCredit))
		body := scheduledBytes(seed, stream.id, stream.offset, size)
		if err := writeWorkerFrame(attachment, workerFrame{kind: frameBytes, id: stream.id, body: body}); err != nil {
			return err
		}
		stream.offset += uint64(size)
		stream.sendCredit -= uint32(size)
		budget -= size
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
