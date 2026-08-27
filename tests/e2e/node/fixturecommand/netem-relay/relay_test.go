package main

import (
	"io"
	"net"
	"testing"
)

func TestCopyRelayDirectionCapsOneDirection(t *testing.T) {
	sourceReader, sourceWriter := net.Pipe()
	destinationReader, destinationWriter := net.Pipe()
	t.Cleanup(func() {
		_ = sourceReader.Close()
		_ = sourceWriter.Close()
		_ = destinationReader.Close()
		_ = destinationWriter.Close()
	})
	result := make(chan struct {
		count int64
		err   error
	}, 1)
	go func() {
		count, err := copyRelayDirection(destinationWriter, sourceReader)
		result <- struct {
			count int64
			err   error
		}{count, err}
	}()
	go func() { _, _ = sourceWriter.Write(make([]byte, relayDirectionByteLimit+1)) }()
	if received, err := io.ReadAll(io.LimitReader(destinationReader, relayDirectionByteLimit)); err != nil || len(received) != relayDirectionByteLimit {
		t.Fatalf("received %d bytes with %v, want %d exact bytes", len(received), err, relayDirectionByteLimit)
	}
	if result := <-result; result.err != nil || result.count != relayDirectionByteLimit {
		t.Fatalf("copy result = (%d, %v), want (%d, nil)", result.count, result.err, relayDirectionByteLimit)
	}
}
