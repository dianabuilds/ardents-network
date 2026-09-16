package streamqualification

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestRetainedCanaryRequiresExactFreshEchoAcrossFragments(t *testing.T) {
	stream := &workerStream{id: 129, sendCredit: 64, receiveCredit: frameCreditWindow}
	schedule := Schedule{ActiveConnections: 0}
	streams := map[uint32]*workerStream{129: stream}
	var output bytes.Buffer
	if err := sendCanaries(&output, streams, []uint32{129}, schedule, time.Now()); err != nil {
		t.Fatal(err)
	}
	frame, err := readWorkerFrame(&output)
	if err != nil || len(frame.body) != 32 || !stream.awaitingCanary {
		t.Fatal("challenge not emitted")
	}
	if err := acceptCanaryBytes(io.Discard, stream, true, frame.body[:7]); err != nil {
		t.Fatal(err)
	}
	if err := requireCompletedCanaries(streams, []uint32{129}, schedule); err == nil {
		t.Fatal("partial echo passed")
	}
	if err := acceptCanaryBytes(io.Discard, stream, true, frame.body[7:]); err != nil {
		t.Fatal(err)
	}
	if err := requireCompletedCanaries(streams, []uint32{129}, schedule); err != nil {
		t.Fatal(err)
	}
	if err := acceptCanaryBytes(io.Discard, stream, true, frame.body); err == nil {
		t.Fatal("replayed echo accepted")
	}
}

func TestRetainedCanaryStallAndOversizedBodyFail(t *testing.T) {
	stream := &workerStream{id: 129, awaitingCanary: true, nextCanary: time.Now().Add(-3 * time.Second)}
	if err := sendCanaries(io.Discard, map[uint32]*workerStream{129: stream}, []uint32{129}, Schedule{}, time.Now()); err == nil {
		t.Fatal("stalled canary passed")
	}
	if err := acceptCanaryBytes(io.Discard, stream, true, make([]byte, 33)); err == nil {
		t.Fatal("oversized echo accepted")
	}
}
