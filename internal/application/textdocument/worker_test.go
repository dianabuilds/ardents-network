//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

type workerWireFrame struct {
	kind byte
	id   uint32
	body []byte
}

func startTextWorker(t *testing.T, mode textdocument.WorkerMode, snapshot []byte) (net.Conn, <-chan error) {
	t.Helper()
	worker, peer := net.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	done := make(chan error, 1)
	go func() { done <- textdocument.RunWorker(ctx, worker, mode); close(done) }()
	t.Cleanup(func() {
		cancel()
		_ = peer.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("worker I/O cleanup did not join")
		}
	})
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 77)
	copy(header, "ARDTWP01")
	header[8] = byte(mode)
	header[9] = 31
	binary.BigEndian.PutUint32(header[41:45], uint32(len(snapshot)))
	digest := sha256.Sum256(snapshot)
	copy(header[45:], digest[:])
	if _, err := peer.Write(append(header, snapshot...)); err != nil {
		t.Fatal(err)
	}
	var readiness [72]byte
	if _, err := io.ReadFull(peer, readiness[:]); err != nil {
		t.Fatal(err)
	}
	if string(readiness[:8]) != "ARDTWR01" || !bytes.Equal(readiness[8:40], header[9:41]) || !bytes.Equal(readiness[40:], digest[:]) {
		t.Fatal("worker readiness lost its nonce or snapshot binding")
	}
	return peer, done
}

func writeWorkerWire(t *testing.T, peer net.Conn, kind byte, id uint32, body []byte) {
	t.Helper()
	header := make([]byte, 9)
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:5], id)
	binary.BigEndian.PutUint32(header[5:], uint32(len(body)))
	if _, err := peer.Write(append(header, body...)); err != nil {
		t.Fatal(err)
	}
}

func readWorkerWire(t *testing.T, peer net.Conn) workerWireFrame {
	t.Helper()
	var header [9]byte
	if _, err := io.ReadFull(peer, header[:]); err != nil {
		t.Fatal(err)
	}
	length := binary.BigEndian.Uint32(header[5:])
	if length > 16384 {
		t.Fatal("worker emitted oversized frame")
	}
	frame := workerWireFrame{kind: header[0], id: binary.BigEndian.Uint32(header[1:5]), body: make([]byte, length)}
	if _, err := io.ReadFull(peer, frame.body); err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestPublisherWorkerSharesSnapshotAcrossIndependentCreditWindows(t *testing.T) {
	document := bytes.Repeat([]byte("abcdefgh"), 20<<10)
	peer, _ := startTextWorker(t, textdocument.PublisherWorker, document)
	request := make([]byte, 512)
	copy(request, "ARDTXT01\x01")
	for _, id := range []uint32{1, 3} {
		writeWorkerWire(t, peer, 1, id, nil)
		writeWorkerWire(t, peer, 2, id, request)
		writeWorkerWire(t, peer, 4, id, nil)
	}
	responses := map[uint32][]byte{1: nil, 3: nil}
	finished := map[uint32]bool{}
	for len(finished) < 2 {
		frame := readWorkerWire(t, peer)
		if _, ok := responses[frame.id]; !ok {
			t.Fatal("worker emitted unsolicited stream")
		}
		switch frame.kind {
		case 2:
			responses[frame.id] = append(responses[frame.id], frame.body...)
			credit := make([]byte, 4)
			binary.BigEndian.PutUint32(credit, uint32(len(frame.body)))
			writeWorkerWire(t, peer, 3, frame.id, credit)
		case 3:
			if len(frame.body) != 4 {
				t.Fatal("worker credit framing is invalid")
			}
		case 4:
			finished[frame.id] = true
		default:
			t.Fatalf("unexpected Publisher frame %d", frame.kind)
		}
	}
	for id, response := range responses {
		if len(response) != 13+len(document) || string(response[:8]) != "ARDTXT01" || response[8] != 0 || !bytes.Equal(response[13:], document) {
			t.Fatalf("stream %d returned different snapshot", id)
		}
	}
}

func TestReaderWorkerWaitsForTerminalBeforeResult(t *testing.T) {
	peer, done := startTextWorker(t, textdocument.ReaderWorker, nil)
	writeWorkerWire(t, peer, 1, 1, nil)
	request := readWorkerWire(t, peer)
	end := readWorkerWire(t, peer)
	if request.kind != 2 || request.id != 1 || len(request.body) != 512 || string(request.body[:9]) != "ARDTXT01\x01" || end.kind != 4 || end.id != 1 {
		t.Fatal("reader did not emit one bounded request")
	}
	writeWorkerWire(t, peer, 2, 1, []byte("ARDTXT01\x00\x00\x00\x00\x05hello"))
	// Consume the replenishment before EOF, then prove EOF is not a result.
	credit := readWorkerWire(t, peer)
	if credit.kind != 3 {
		t.Fatal("reader did not release consumed transport space")
	}
	writeWorkerWire(t, peer, 4, 1, nil)
	if err := peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var unexpected [1]byte
	if n, err := peer.Read(unexpected[:]); err == nil || n != 0 {
		t.Fatal("reader emitted result before authenticated terminal")
	}
	if err := peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	writeWorkerWire(t, peer, 5, 1, []byte{0})
	result := readWorkerWire(t, peer)
	body := readWorkerWire(t, peer)
	eof := readWorkerWire(t, peer)
	if result.kind != 6 || result.id != 2 || !bytes.Equal(result.body, []byte{0, 0, 0, 0, 5}) || body.kind != 2 || body.id != 2 || string(body.body) != "hello" || eof.kind != 4 || eof.id != 2 {
		t.Fatal("reader result framing is invalid")
	}
	writeWorkerWire(t, peer, 5, 2, []byte{0})
	if err := <-done; err != nil {
		t.Fatalf("reader completion = %v", err)
	}
}

func TestTextWorkerRefusesUnsolicitedStreamsAndCreditOverflow(t *testing.T) {
	for _, test := range []struct {
		name string
		kind byte
		id   uint32
		body []byte
	}{
		{"unsolicited bytes", 2, 3, []byte("x")},
		{"even open", 1, 2, nil},
		{"overflow", 3, 1, []byte{0, 0, 0, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer, done := startTextWorker(t, textdocument.PublisherWorker, []byte("private"))
			writeWorkerWire(t, peer, 1, 1, nil)
			writeWorkerWire(t, peer, test.kind, test.id, test.body)
			if err := <-done; err == nil {
				t.Fatal("malformed worker exchange was accepted")
			}
		})
	}
}

func TestPublisherRejectsOnlyMalformedDocumentStream(t *testing.T) {
	for _, size := range []int{1, 513, 1024} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			document := bytes.Repeat([]byte("a"), 160<<10)
			peer, _ := startTextWorker(t, textdocument.PublisherWorker, document)
			request := make([]byte, 512)
			copy(request, "ARDTXT01\x01")
			writeWorkerWire(t, peer, 1, 1, nil)
			writeWorkerWire(t, peer, 2, 1, request)
			writeWorkerWire(t, peer, 4, 1, nil)
			writeWorkerWire(t, peer, 1, 3, nil)
			writeWorkerWire(t, peer, 2, 3, bytes.Repeat([]byte("x"), size))
			writeWorkerWire(t, peer, 4, 3, nil)
			var response []byte
			goodEOF, refused := false, false
			for !goodEOF || !refused {
				frame := readWorkerWire(t, peer)
				if frame.id == 3 {
					if frame.kind == 3 {
						continue // Credit already queued before detecting invalid input.
					}
					if frame.kind != 5 || len(frame.body) != 1 || frame.body[0] == 0 || refused {
						t.Fatalf("malformed stream emitted %+v", frame)
					}
					refused = true
					writeWorkerWire(t, peer, 5, 3, []byte{1})
					continue
				}
				if frame.id != 1 {
					t.Fatalf("foreign stream %d", frame.id)
				}
				switch frame.kind {
				case 2:
					response = append(response, frame.body...)
					var credit [4]byte
					binary.BigEndian.PutUint32(credit[:], uint32(len(frame.body)))
					writeWorkerWire(t, peer, 3, 1, credit[:])
				case 3:
				case 4:
					goodEOF = true
				default:
					t.Fatalf("honest stream terminated: %+v", frame)
				}
			}
			if len(response) != 13+len(document) || !bytes.Equal(response[13:], document) {
				t.Fatal("malformed neighbor interrupted the honest document")
			}
		})
	}
}
