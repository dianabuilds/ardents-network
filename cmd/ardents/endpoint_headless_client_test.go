package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHeadlessOpenCarriesBytesThroughOnlyTheServiceLinkInterface(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "aho-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seen := make(chan struct {
		link, input string
	}, 1)
	go serveHeadlessOpenFixture(listener, seen)
	inputPath, outputPath := filepath.Join(t.TempDir(), "request"), filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(inputPath, []byte("request bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	var receipt bytes.Buffer
	if err := runHeadlessOpen(t.Context(), socket, "ardents-alpha://service.alice", inputPath, outputPath, &receipt); err != nil {
		t.Fatal(err)
	}
	observed := <-seen
	if observed.link != "ardents-alpha://service.alice" || observed.input != "request bytes" {
		t.Fatalf("headless open observed %+v", observed)
	}
	response, err := os.ReadFile(outputPath)
	if err != nil || string(response) != "response bytes" {
		t.Fatalf("headless response = %q, %v", response, err)
	}
	if !bytes.Contains(receipt.Bytes(), []byte("headless-open-complete")) || bytes.Contains(receipt.Bytes(), []byte("target")) {
		t.Fatalf("headless open receipt = %s", receipt.Bytes())
	}
}

func serveHeadlessOpenFixture(listener net.Listener, seen chan<- struct{ link, input string }) {
	connection, err := listener.Accept()
	if err != nil {
		seen <- struct{ link, input string }{}
		return
	}
	defer connection.Close()
	header := make([]byte, 6)
	_, _ = io.ReadFull(connection, header)
	link := make([]byte, int(binary.BigEndian.Uint16(header[4:])))
	_, _ = io.ReadFull(connection, link)
	_, _ = connection.Write([]byte{1})
	var input bytes.Buffer
	for {
		var frame [4]byte
		_, _ = io.ReadFull(connection, frame[:])
		length := binary.BigEndian.Uint32(frame[:])
		if length == 0 {
			break
		}
		_, _ = io.CopyN(&input, connection, int64(length))
	}
	seen <- struct{ link, input string }{string(link), input.String()}
	response := []byte("response bytes")
	var frame [4]byte
	binary.BigEndian.PutUint32(frame[:], uint32(len(response)))
	_, _ = connection.Write(append(frame[:], response...))
	class, reason := []byte("clean service connection close"), []byte("fixture complete")
	terminal := make([]byte, 8+len(class)+len(reason))
	binary.BigEndian.PutUint32(terminal[:4], ^uint32(0))
	binary.BigEndian.PutUint16(terminal[4:6], uint16(len(class)))
	binary.BigEndian.PutUint16(terminal[6:8], uint16(len(reason)))
	copy(terminal[8:], class)
	copy(terminal[8+len(class):], reason)
	_, _ = connection.Write(terminal)
}

func TestHeadlessAdministrationUsesOnlyTheSelectedLocalOperation(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "ahc-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	request := make(chan string, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			request <- ""
			return
		}
		defer connection.Close()
		raw := make([]byte, 9)
		_, _ = io.ReadFull(connection, raw)
		request <- string(raw)
		_, _ = connection.Write([]byte("withdrawn\n"))
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var output bytes.Buffer
	if err := runHeadlessAdministration(ctx, "withdraw", socket, &output); err != nil {
		t.Fatal(err)
	}
	if got := <-request; got != "withdraw\n" {
		t.Fatalf("administration request = %q", got)
	}
	if !bytes.Contains(output.Bytes(), []byte("headless-service-withdrawn")) {
		t.Fatalf("administration output = %s", output.Bytes())
	}
}
