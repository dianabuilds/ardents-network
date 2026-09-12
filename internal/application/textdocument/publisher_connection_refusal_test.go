//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

type hostilePublisherAttachment struct {
	net.Conn
	ready chan struct{}
	once  sync.Once
	input *bytes.Reader
}

func (attachment *hostilePublisherAttachment) Write(body []byte) (int, error) {
	attachment.once.Do(func() { close(attachment.ready) })
	return attachment.Conn.Write(body)
}
func (attachment *hostilePublisherAttachment) Read(body []byte) (int, error) {
	<-attachment.ready
	if attachment.input.Len() != 0 {
		return attachment.input.Read(body)
	}
	return attachment.Conn.Read(body)
}
func (attachment *hostilePublisherAttachment) Close() error {
	attachment.once.Do(func() { close(attachment.ready) })
	return attachment.Conn.Close()
}

func TestPublisherWorkerRejectsHostileFramesAndClosesAdmittedService(t *testing.T) {
	for _, test := range []struct {
		name string
		kind byte
		id   uint32
		size uint32
		body []byte
	}{
		{name: "worker open", kind: 1, id: 1},
		{name: "unsolicited stream", kind: 4, id: 3},
		{name: "reader result", kind: 6, id: 1, size: 5, body: make([]byte, 5)},
		{name: "credit overflow", kind: 3, id: 1, size: 4, body: []byte{0, 1, 0, 1}},
		{name: "oversized frame without payload", kind: 2, id: 1, size: 16385},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := make([]byte, 9)
			header[0] = test.kind
			binary.BigEndian.PutUint32(header[1:5], test.id)
			binary.BigEndian.PutUint32(header[5:], test.size)
			harness := startPublisherHarness(t, []byte("snapshot"), func(conn net.Conn) io.ReadWriteCloser {
				return &hostilePublisherAttachment{Conn: conn, ready: make(chan struct{}), input: bytes.NewReader(append(header, test.body...))}
			})
			stream := newPublisherFixture(publisherRequest(), nil)
			harness.admit(t, stream)
			select {
			case <-harness.finished:
				if harness.err == nil {
					t.Fatal("hostile worker frame was accepted")
				}
				select {
				case <-stream.closed:
				default:
					t.Fatal("hostile worker failure retained the Service")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("hostile worker stalled Publisher cleanup")
			}
		})
	}
}

func TestPublisherWorkerAlreadyCancelledDoesNotReceiveAnotherService(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	endpoint, peer := net.Pipe()
	defer peer.Close()
	stream := newPublisherFixture(publisherRequest(), nil)
	defer stream.Close()
	incoming := make(chan connection.Stream, 1)
	incoming <- stream
	if err := textdocument.ServeWorkerConnections(ctx, endpoint, incoming); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel outcome = %v", err)
	}
	if len(incoming) != 1 {
		t.Fatal("cancelled job received another Service")
	}
	select {
	case <-stream.closed:
		t.Fatal("unreceived Service ownership moved to cancelled job")
	default:
	}
	var body [1]byte
	if _, err := peer.Read(body[:]); err != io.EOF {
		t.Fatalf("cancelled job retained its worker attachment: %v", err)
	}
}
