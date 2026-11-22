//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
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
	snapshot, err := textdocument.NewSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	owner := &snapshotConnectionOwner{snapshot: snapshot}
	address := filepath.Join(directory, "connection.sock")
	server, err := connection.Listen(address, owner)
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
	stream, err := connection.Dial(ctx, address, connection.Request{Destination: connection.TargetLink, Value: "fixture-link"})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := startTextWorker(t, textdocument.ReaderWorker, nil)
	actual, err := readTransferredReader(ctx, worker, stream)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, body) || owner.opens.Load() != 1 || owner.bytes.Load() != 512 {
		t.Fatal("AAI3 exchange lost its exact content or one-request binding")
	}
}
