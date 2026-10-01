package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHeadlessAdministrationUsesOnlyTheSelectedLocalOperation(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "ahc-"+time.Now().Format("150405.000000")+".sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	type peerResult struct {
		request string
		err     error
	}
	peerReady := make(chan net.Conn, 1)
	result := make(chan peerResult, 1)
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			result <- peerResult{err: acceptErr}
			return
		}
		peerReady <- connection
		raw := make([]byte, 9)
		_, readErr := io.ReadFull(connection, raw)
		var writeErr error
		if readErr == nil {
			_, writeErr = connection.Write([]byte("withdrawn\n"))
		}
		result <- peerResult{request: string(raw), err: errors.Join(readErr, writeErr, connection.Close())}
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Administration listener: %v", err)
		}
		select {
		case connection := <-peerReady:
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("close Administration peer: %v", err)
			}
		case <-peerDone:
		}
		select {
		case <-peerDone:
		case <-time.After(time.Second):
			t.Error("Administration peer goroutine did not stop")
		}
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove Administration socket: %v", err)
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Errorf("Administration socket residue: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var output bytes.Buffer
	if err := runHeadlessAdministration(ctx, "withdraw", socket, &output); err != nil {
		t.Fatal(err)
	}
	observed := <-result
	<-peerDone
	if observed.err != nil || observed.request != "withdraw\n" {
		t.Fatalf("administration request = %q, %v", observed.request, observed.err)
	}
	if !bytes.Contains(output.Bytes(), []byte("headless-service-withdrawn")) {
		t.Fatalf("administration output = %s", output.Bytes())
	}
}
