package main

import (
	"io"
	"net"
	"testing"
)

func TestPublicationOperatorUsesOnlyAdministrationRequest(t *testing.T) {
	application, endpoint := net.Pipe()
	defer application.Close()
	defer endpoint.Close()
	done := make(chan error, 1)
	go func() { done <- publish(application) }()
	request := make([]byte, 8)
	if _, err := io.ReadFull(endpoint, request); err != nil || string(request) != "publish\n" {
		t.Fatalf("unexpected administration request: %q %v", request, err)
	}
	if _, err := endpoint.Write([]byte("published\n")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
