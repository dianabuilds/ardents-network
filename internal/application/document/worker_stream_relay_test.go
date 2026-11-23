package document

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestByteStreamWorkerRelaysBothDirectionsAndHalfClose(t *testing.T) {
	var input, output bytes.Buffer
	credit := binary.BigEndian.AppendUint32(nil, 2)
	frames := []workerFrame{
		{kind: 1, id: 1}, {kind: 1, id: 3},
		{kind: 2, id: 1, body: []byte{0, 255}}, {kind: 3, id: 3, body: credit},
		{kind: 4, id: 1}, {kind: 2, id: 3, body: []byte{129, 0}},
		{kind: 4, id: 3}, {kind: 5, id: 1, body: []byte{0}},
	}
	for _, frame := range frames {
		if err := writeWorkerFrame(&input, frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := relayWorkerStreams(struct {
		io.Reader
		io.Writer
	}{&input, &output}); err != nil {
		t.Fatal(err)
	}
	for _, sent := range frames[2:7] {
		got, err := readWorkerFrame(&output)
		if err != nil || got.kind != sent.kind || got.id != 4-sent.id || !bytes.Equal(got.body, sent.body) {
			t.Fatalf("frame %+v did not cross to the other attachment: %+v, %v", sent, got, err)
		}
	}
	if output.Len() != 0 {
		t.Fatal("unexpected additional worker output")
	}
}

func TestInstalledRoleWorkerSelectsStreamByExplicitInitialization(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	host, isolated := net.Pipe()
	defer host.Close()
	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, isolated, ReaderWorker) }()

	if err := InitializeWorker(ctx, host, ByteStreamWorker, [32]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	for _, frame := range []workerFrame{
		{kind: 1, id: 1}, {kind: 1, id: 3},
		{kind: 2, id: 1, body: []byte{255}},
	} {
		if err := writeWorkerFrame(host, frame); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := readWorkerFrame(host); err != nil || got.id != 3 || !bytes.Equal(got.body, []byte{255}) {
		t.Fatalf("installed role did not select its relay: %+v, %v", got, err)
	}
	_ = host.Close()
	if err := <-done; err == nil {
		t.Fatal("unconfirmed completion was accepted")
	}
}

func TestByteStreamWorkerBridgesIndependentPorts(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	host, isolated := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, isolated, PublisherWorker) }()
	if err := InitializeWorker(ctx, host, ByteStreamWorker, [32]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	firstBytes := bytes.Repeat([]byte{255, 0, 1}, 12000)
	secondBytes := bytes.Repeat([]byte{129, 0}, 21000)
	first := &relayPort{Reader: bytes.NewReader(firstBytes)}
	second := &relayPort{Reader: bytes.NewReader(secondBytes)}
	if err := BridgeByteStreamWorker(ctx, host, first, second); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.received.Bytes(), secondBytes) || !bytes.Equal(second.received.Bytes(), firstBytes) {
		t.Fatal("independent attachments did not receive exact bytes")
	}
	if !first.eof || !second.eof {
		t.Fatal("both ports did not receive directional EOF")
	}
}

type relayPort struct {
	*bytes.Reader
	received bytes.Buffer
	eof      bool
}

func (port *relayPort) Write(body []byte) (int, error) { return port.received.Write(body) }
func (port *relayPort) CloseInput() error {
	port.eof = true
	return nil
}
func (*relayPort) Close() error { return nil }

func TestByteStreamFailureDoesNotExposeUnderlyingDiagnostic(t *testing.T) {
	for _, category := range []error{errWorkerRelayProtocol, errWorkerRelayRead, errWorkerRelayWrite} {
		got := ByteStreamFailure(errors.Join(category, errors.New("private input")))
		if got != category.Error() {
			t.Fatalf("failure projection %q differs from %q", got, category)
		}
	}
}
