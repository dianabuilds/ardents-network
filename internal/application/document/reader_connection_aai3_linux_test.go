//go:build linux

package document_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// The AAI3 transfer control uses the selected Linux Connection listener; the
// portable reader controls in reader_connection_test.go exercise the same
// retained contract over admitted fixture streams on every platform.
func TestReaderWorkerTransfersThroughAAI3WithoutSecondOpen(t *testing.T) {
	directory, err := os.MkdirTemp("", "text-aai3-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(directory); err != nil {
			t.Error(err)
		}
	})
	body := bytes.Repeat([]byte("closed text\n"), 8192)
	owner := &snapshotConnectionOwner{snapshot: body}
	address := filepath.Join(directory, "connection.sock")
	server, err := endpointstream.Listen(address, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream, err := endpointstream.Dial(ctx, address, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "fixture-link"})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := startDocumentWorker(t, document.ReaderWorker, nil)
	actual, err := readTransferredReader(ctx, worker, stream)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, body) || owner.opens.Load() != 1 || owner.bytes.Load() != 0 {
		t.Fatal("AAI3 exchange lost its exact content or one-request binding")
	}
}
