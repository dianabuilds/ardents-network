package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEndpointBodylessPublishRefusesBeforeAdministrationDial(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "apr-"+time.Now().Format("150405.000000")+".sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan bool, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			accepted <- false
			return
		}
		_ = connection.Close()
		accepted <- true
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var output bytes.Buffer
	err = runEndpoint(ctx, []string{"endpoint", "publish", socket}, &output)
	if closeErr := listener.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if <-accepted {
		t.Error("bodyless publish dialed the Administration socket")
	}
	const refusal = "endpoint publish is retired; use ardents-text publish <administration-socket> <document-file>"
	if err == nil || err.Error() != refusal {
		t.Errorf("bodyless publication = %v", err)
	}
	if output.Len() != 0 {
		t.Errorf("bodyless publication emitted success: %q", output.String())
	}
}
