package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEndpointOpenRefusesBeforeFileOrIPCEffects(t *testing.T) {
	const refusal = "endpoint open is retired"
	root := t.TempDir()
	missingInput := filepath.Join(root, "missing-input")
	output := filepath.Join(root, "response")
	arguments := []string{"endpoint", "open", filepath.Join(root, "missing.sock"), "ardents-target:v1:retired", missingInput, output}
	if err := runEndpoint(t.Context(), arguments, io.Discard); err == nil || err.Error() != refusal {
		t.Fatalf("retired endpoint open with missing input = %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("retired endpoint open created output: %v", err)
	}

	input := filepath.Join(root, "request")
	if err := os.WriteFile(input, []byte("unchanged request"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("unchanged response"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments[4], arguments[5] = input, output
	if err := runEndpoint(t.Context(), arguments, io.Discard); err == nil || err.Error() != refusal {
		t.Fatalf("retired endpoint open with existing output = %v", err)
	}
	if got, err := os.ReadFile(input); err != nil || !bytes.Equal(got, []byte("unchanged request")) {
		t.Fatalf("retired endpoint open changed input = %q, %v", got, err)
	}
	if got, err := os.ReadFile(output); err != nil || !bytes.Equal(got, []byte("unchanged response")) {
		t.Fatalf("retired endpoint open changed output = %q, %v", got, err)
	}

	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("aor-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove retired-open socket: %v", err)
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Errorf("retired-open socket residue: %v", err)
		}
	})
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan bool, 1)
	peerClose := make(chan error, 1)
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			accepted <- false
			peerClose <- nil
			return
		}
		accepted <- true
		peerClose <- connection.Close()
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close retired-open listener: %v", err)
		}
		select {
		case <-peerDone:
		case <-time.After(time.Second):
			t.Error("retired-open peer goroutine did not stop")
			return
		}
		if err := <-peerClose; err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close unexpected retired-open peer: %v", err)
		}
	})
	arguments[2], arguments[5] = socket, output
	if err := runEndpoint(t.Context(), arguments, io.Discard); err == nil || err.Error() != refusal {
		t.Fatalf("retired endpoint open with available IPC = %v", err)
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
	<-peerDone
	if <-accepted {
		t.Fatal("retired endpoint open connected to its Application socket")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("retired endpoint open created output before IPC refusal: %v", err)
	}
}
