//go:build linux

package connection

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func TestByteStreamResponseEOFPreservesSending(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	deadline := time.Now().Add(time.Second)
	_ = local.SetDeadline(deadline)
	_ = peer.SetDeadline(deadline)
	stream := newClientWithStop(halfCloseTestTransport{local}, nil)
	stream.directionalEOF = true
	go stream.receive()
	defer stream.Close()

	terminal := make(chan error, 1)
	go func() {
		if _, err := peer.Write([]byte{0, 0, 0, 0}); err != nil {
			terminal <- err
			return
		}
		var frame [6]byte
		if _, err := io.ReadFull(peer, frame[:]); err != nil {
			terminal <- err
			return
		}
		if !bytes.Equal(frame[:], []byte{0, 0, 0, 2, 255, 0}) {
			terminal <- io.ErrUnexpectedEOF
			return
		}
		terminal <- writeTerminal(peer, Outcome{Class: CleanClose, Reason: "complete"})
	}()

	if _, err := stream.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("response direction did not end independently: %v", err)
	}
	select {
	case <-stream.Done():
		t.Fatal("directional EOF published a terminal outcome")
	default:
	}
	if _, err := stream.Write([]byte{255, 0}); err != nil {
		t.Fatal(err)
	}
	if err := <-terminal; err != nil {
		t.Fatal(err)
	}
	if result := <-stream.Done(); result.Class != CleanClose {
		t.Fatal(result)
	}
}

type halfCloseTestTransport struct{ net.Conn }

func (halfCloseTestTransport) CloseWrite() error { return nil }
