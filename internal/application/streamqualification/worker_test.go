package streamqualification

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSenderUsesOnlyEndpointOpenedCreditedStreams(t *testing.T) {
	endpoint, worker := net.Pipe()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(ctx, worker, Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}})
	}()
	schedule, err := ClientToPublisher.Definition(ReaderRole)
	if err != nil {
		t.Fatal(err)
	}
	for index := uint32(0); index < uint32(schedule.OpenConnections); index++ {
		id := 1 + 2*index
		if index >= 16 {
			id = 129 + 2*(index-16)
		}
		writeFrame(t, endpoint, workerFrame{kind: frameOpen, id: id})
		var credit [4]byte
		binary.BigEndian.PutUint32(credit[:], frameCreditWindow)
		writeFrame(t, endpoint, workerFrame{kind: frameCredit, id: id, body: credit[:]})
	}
	_ = endpoint.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := readWorkerFrame(endpoint)
	for err == nil && frame.id > 127 {
		frame, err = readWorkerFrame(endpoint)
	}
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

func TestScheduleGivesEveryActiveStreamUsefulBytes(t *testing.T) {
	for _, role := range []Role{ReaderRole, PublisherRole} {
		schedule, err := ClientToPublisher.Definition(role)
		if err != nil {
			t.Fatal(err)
		}
		streams := make(map[uint32]*workerStream, schedule.OpenConnections)
		order := make([]uint32, 0, schedule.OpenConnections)
		for index := uint16(0); index < schedule.OpenConnections; index++ {
			id := uint32(index)*2 + 1
			streams[id] = &workerStream{id: id}
			order = append(order, id)
		}
		var output bytes.Buffer
		for tick := 0; tick < 400; tick++ {
			for _, stream := range streams {
				stream.sendCredit = frameCreditWindow
			}
			if err := sendScheduledElapsed(&output, streams, order, [32]byte{2}, schedule, time.Duration(tick+1)*scheduleTick); err != nil {
				t.Fatal(err)
			}
			output.Reset()
		}
		elapsed := 400 * scheduleTick
		for index, id := range order {
			if index < int(schedule.ActiveConnections) {
				bitrate := uint64(streams[id].offset) * 8 * uint64(time.Second) / uint64(elapsed)
				if bitrate < 500_000 {
					t.Fatalf("role %d active stream %d useful bitrate = %d bit/s", role, id, bitrate)
				}
			}
			if index >= int(schedule.ActiveConnections) && streams[id].offset != 0 {
				t.Fatalf("role %d used canary stream %d as workload", role, id)
			}
		}
	}
}

func TestScheduledStreamAcceptsReadFragmentation(t *testing.T) {
	init := Init{Role: ReaderRole, Profile: PublisherToClient, Nonce: [32]byte{1}, Seed: [32]byte{2}}
	schedule, err := init.Profile.Definition(init.Role)
	if err != nil {
		t.Fatal(err)
	}
	streams := map[uint32]*workerStream{1: {id: 1, receiveCredit: frameCreditWindow}}
	order := []uint32{1}
	lastID := uint32(1)
	var output bytes.Buffer
	body := scheduledBytes(init.Seed, 1, 0, 100)
	for _, part := range [][]byte{body[:13], body[13:]} {
		if err := acceptWorkerFrame(&output, streams, &order, &lastID, init, schedule, false, workerFrame{kind: frameBytes, id: 1, body: part}); err != nil {
			t.Fatalf("valid ordered bytes were rejected after %d bytes: %v", streams[1].offset, err)
		}
	}
}

func TestScheduledCorpusCoversAllBytesAcrossFragmentBoundaries(t *testing.T) {
	seed := [32]byte{2}
	const streamID = uint32(1)
	body := scheduledBytes(seed, streamID, 0, frameLimit)
	var seen [256]bool
	for _, value := range body {
		seen[value] = true
	}
	for value, present := range seen {
		if !present {
			t.Fatalf("fixed qualification corpus omitted byte 0x%02x", value)
		}
	}
	if utf8.Valid(body) {
		t.Fatal("fixed qualification corpus unexpectedly contains only UTF-8")
	}
	boundaries := []int{0, 1, 257, 4093, len(body)}
	for index := 1; index < len(boundaries); index++ {
		start, end := boundaries[index-1], boundaries[index]
		if !matchesScheduledBytes(seed, streamID, uint64(start), body[start:end]) {
			t.Fatalf("exact corpus fragment %d:%d was rejected", start, end)
		}
	}
	corrupted := append([]byte(nil), body...)
	corrupted[257] ^= 0xff
	if matchesScheduledBytes(seed, streamID, 0, corrupted) {
		t.Fatal("corrupted qualification corpus was accepted")
	}
	if matchesScheduledBytes(seed, streamID, 1, body) {
		t.Fatal("qualification corpus was accepted at the wrong stream offset")
	}
}
