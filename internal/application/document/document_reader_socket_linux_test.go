//go:build linux

package document_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointserver "github.com/dianabuilds/ardents-network/internal/application/endpointstream/server"
)

func documentReadSocket(t *testing.T, peer endpointstream.Interface) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "document-read-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(dir); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(dir, "local.sock")
	server, err := endpointserver.Listen(socket, peer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return socket
}
