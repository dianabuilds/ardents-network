package main

import (
	"bytes"
	"net"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func TestExternalApplicationsExchangeOpaqueBytesWithoutArdentsState(t *testing.T) {
	client, publisher := net.Pipe()
	defer client.Close()
	defer publisher.Close()
	type outcome struct {
		value observation
		err   error
	}
	results := make(chan outcome, 2)
	go func() {
		value, err := exchange(client, "client", [32]byte{17}, [32]byte{91}, 4096, 4096, nil, nil)
		results <- outcome{value, err}
	}()
	go func() {
		value, err := exchange(publisher, "publisher", [32]byte{91}, [32]byte{17}, 4096, 4096, nil, nil)
		results <- outcome{value, err}
	}()
	for range 2 {
		result := <-results
		if result.err != nil || result.value.Terminal != "success" || result.value.SentBytes != 4096 || result.value.ReceivedBytes != 4096 {
			t.Fatalf("opaque external Application failed: value=%+v err=%v", result.value, result.err)
		}
	}
}

func TestPacedWorkloadWriterUsesNonRecordAlignedFiniteChunks(t *testing.T) {
	var output bytes.Buffer
	writer, err := workloadWriter(&output, "1ms")
	if err != nil {
		t.Fatal(err)
	}
	written, err := writer(make([]byte, 32<<10))
	if err != nil || written != 16_381 || output.Len() != 16_381 {
		t.Fatalf("paced write=%d retained=%d err=%v", written, output.Len(), err)
	}
	if _, err := workloadWriter(&output, "101ms"); err == nil {
		t.Fatal("unbounded stream pacing accepted")
	}
}

func TestExternalApplicationRequiresClassifiedConnectionResult(t *testing.T) {
	application, endpoint := net.Pipe()
	defer application.Close()
	defer endpoint.Close()
	go func() {
		_ = applicationipc.Write(endpoint, applicationipc.Result{Class: "clean service connection close",
			AuthenticatedTarget: [32]byte{1}, AcceptedBytes: 4096, ReceivedBytes: 4096})
	}()
	result, err := applicationipc.Read(application)
	if err != nil || result.Class != "clean service connection close" || result.AcceptedBytes != 4096 {
		t.Fatalf("classified result=%+v err=%v", result, err)
	}

	cleanEOF, peer := net.Pipe()
	_ = peer.Close()
	defer cleanEOF.Close()
	if _, err := applicationipc.Read(cleanEOF); err == nil {
		t.Fatal("clean EOF was treated as semantic Application success")
	}
}
