package streamqualification

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

// Canaries are fresh random challenges on retained authenticated streams.
// Only their echoes complete; seed predictability cannot manufacture progress.
func sendCanaries(output io.Writer, streams map[uint32]*workerStream, order []uint32, schedule Schedule, now time.Time) error {
	for _, id := range order[int(schedule.ActiveConnections):] {
		stream := streams[id]
		if stream == nil || stream.sentEOF {
			continue
		}
		if stream.awaitingCanary {
			if now.Sub(stream.nextCanary) > 2*time.Second {
				return errors.New("retained qualification stream stopped making canary progress")
			}
			continue
		}
		if now.Before(stream.nextCanary) || stream.sendCredit < 32 {
			continue
		}
		if _, err := rand.Read(stream.challenge[:]); err != nil {
			return err
		}
		if err := writeWorkerFrame(output, workerFrame{kind: frameBytes, id: id, body: stream.challenge[:]}); err != nil {
			return err
		}
		stream.sendCredit -= 32
		stream.awaitingCanary = true
		stream.nextCanary = now
	}
	return nil
}

func acceptCanaryBytes(output io.Writer, stream *workerStream, sender bool, body []byte) error {
	if stream.receivedEOF || len(body) == 0 || len(body) > 32-len(stream.canaryBuffer) {
		return errors.New("qualification canary framing invalid")
	}
	stream.canaryBuffer = append(stream.canaryBuffer, body...)
	var credit [4]byte
	binary.BigEndian.PutUint32(credit[:], uint32(len(body)))
	if err := writeWorkerFrame(output, workerFrame{kind: frameCredit, id: stream.id, body: credit[:]}); err != nil {
		return err
	}
	if len(stream.canaryBuffer) != 32 {
		return nil
	}
	if sender {
		if !stream.awaitingCanary || !bytes.Equal(stream.canaryBuffer, stream.challenge[:]) {
			return errors.New("qualification canary echo invalid")
		}
		stream.awaitingCanary = false
		// Fresh challenge bytes choose the next interval, independently per stream.
		stream.nextCanary = time.Now().Add(time.Duration(250+binary.BigEndian.Uint16(stream.challenge[:2])%750) * time.Millisecond)
	} else {
		if stream.sendCredit < 32 {
			return errors.New("qualification canary echo credit exhausted")
		}
		if err := writeWorkerFrame(output, workerFrame{kind: frameBytes, id: stream.id, body: stream.canaryBuffer}); err != nil {
			return err
		}
		stream.sendCredit -= 32
	}
	stream.canaryBuffer = stream.canaryBuffer[:0]
	return nil
}

func requireCompletedCanaries(streams map[uint32]*workerStream, order []uint32, schedule Schedule) error {
	for _, id := range order[int(schedule.ActiveConnections):] {
		if stream := streams[id]; stream == nil || stream.awaitingCanary || len(stream.canaryBuffer) != 0 {
			return errors.New("qualification ended with incomplete canary")
		}
	}
	return nil
}
