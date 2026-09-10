//go:build linux

package textdocument_test

import (
	"bytes"
	"io"
	"net"
	"testing"
)

func TestPublisherWorkerOneByteResponseFramesDoNotBlockProgress(t *testing.T) {
	body := bytes.Repeat([]byte("f"), (64<<10)+1)
	harness := startPublisherHarnessWithWorker(t, body, nil, func(conn net.Conn) io.ReadWriteCloser {
		// This fixture splits real worker output for ID 1 without changing
		// data, frame direction or credit. ID 3 shares the same attachment.
		return &fragmentedRequestAttachment{Conn: conn}
	})
	first := newPublisherFixture(publisherRequest(), nil)
	second := newPublisherFixture(publisherRequest(), nil)
	harness.admit(t, first)
	harness.admit(t, second)
	harness.drain(t)
	first.assertResponse(t, body)
	second.assertResponse(t, body)
}
