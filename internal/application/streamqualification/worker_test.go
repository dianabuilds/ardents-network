package streamqualification

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestSenderUsesOnlyEndpointOpenedCreditedStreams(t *testing.T) {
	endpoint, worker := net.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(ctx, worker, Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}})
	}()
	for index := uint32(0); index < 16; index++ {
		id := 1 + 2*index
		writeFrame(t, endpoint, workerFrame{kind: frameOpen, id: id})
		var credit [4]byte
		binary.BigEndian.PutUint32(credit[:], frameCreditWindow)
		writeFrame(t, endpoint, workerFrame{kind: frameCredit, id: id, body: credit[:]})
	}
	_ = endpoint.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := readWorkerFrame(endpoint)
	if err != nil || frame.kind != frameBytes || frame.id == 0 || !matchesScheduledBytes([32]byte{2}, frame.id, 0, frame.body) {
		t.Fatalf("scheduled bytes: %#v / %v", frame, err)
	}
	cancel()
	_ = endpoint.Close()
	if err := <-done; err == nil {
		t.Fatal("cancelled worker completed successfully")
	}
}

func TestReceiverRejectsChangedScheduledBytesAndReturnsCredit(t *testing.T) {
	endpoint, worker := net.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(ctx, worker, Init{Role: ReaderRole, Profile: PublisherToClient, Nonce: [32]byte{1}, Seed: [32]byte{3}})
	}()
	writeFrame(t, endpoint, workerFrame{kind: frameOpen, id: 1})
	body := scheduledBytes([32]byte{3}, 1, 0, 16)
	writeFrame(t, endpoint, workerFrame{kind: frameBytes, id: 1, body: body})
	_ = endpoint.SetReadDeadline(time.Now().Add(time.Second))
	credit, err := readWorkerFrame(endpoint)
	if err != nil || credit.kind != frameCredit || credit.id != 1 || binary.BigEndian.Uint32(credit.body) != uint32(len(body)) {
		t.Fatalf("credit: %#v / %v", credit, err)
	}
	body[0] ^= 1
	writeFrame(t, endpoint, workerFrame{kind: frameBytes, id: 1, body: body})
	_ = endpoint.Close()
	if err := <-done; err == nil {
		t.Fatal("changed scheduled bytes were accepted")
	}
}

func writeFrame(t *testing.T, writer net.Conn, frame workerFrame) {
	t.Helper()
	_ = writer.SetWriteDeadline(time.Now().Add(time.Second))
	if err := writeWorkerFrame(writer, frame); err != nil {
		t.Fatal(err)
	}
}
