package main

import (
	"io"
	"net"
	"testing"
	"time"

	endpointpkg "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

func TestStreamLifetimeIsBoundedIndependentlyFromDial(t *testing.T) {
	t.Setenv("ARDENTS_STREAM_LIFETIME", "12m")
	if lifetime, err := streamLifetime(); err != nil || lifetime != 12*time.Minute {
		t.Fatalf("stream lifetime=%v err=%v", lifetime, err)
	}
	t.Setenv("ARDENTS_STREAM_LIFETIME", "31m")
	if _, err := streamLifetime(); err == nil {
		t.Fatal("unbounded stream lifetime was accepted")
	}
}

func TestEarlyFailureResultInterruptsIncompleteRawWorkload(t *testing.T) {
	application, endpoint := net.Pipe()
	applicationResult, endpointResult := net.Pipe()
	accepted := make(chan error, 1)
	go func() { _, err := io.ReadFull(endpoint, make([]byte, 6)); accepted <- err }()
	stream, err := endpointpkg.OpenApplication(application, applicationResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	defer endpoint.Close()
	classified := waitForResult(stream)
	exchange := make(chan error, 1)
	go func() {
		_, err := streamworkload.Exchange(stream, "client", [32]byte{1}, [32]byte{2},
			0, 64<<10, nil, nil)
		exchange <- err
	}()
	go func() {
		_ = endpointpkg.Write(endpointResult, endpointpkg.Result{Class: "abrupt connection loss",
			Reason: "route Attachment proposal limit or recovery deadline reached"})
	}()
	select {
	case err := <-exchange:
		if err == nil {
			t.Fatal("incomplete workload was accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("early classified failure did not interrupt the raw workload")
	}
	outcome := <-classified
	if outcome.err != nil || outcome.result.Class != "abrupt connection loss" {
		t.Fatalf("classified outcome=%+v error=%v", outcome.result, outcome.err)
	}
}

func TestStreamCountsPermitBoundedSustainedWorkload(t *testing.T) {
	send, receive, err := streamCounts("268435456", "0")
	if err != nil || send != 256<<20 || receive != 0 {
		t.Fatalf("sustained stream counts=%d/%d err=%v", send, receive, err)
	}
	if _, _, err := streamCounts("268435457", "0"); err == nil {
		t.Fatal("stream count above the product bound was accepted")
	}
}

func TestExternalApplicationRequiresClassifiedConnectionResult(t *testing.T) {
	application, endpoint := net.Pipe()
	applicationResult, endpointResult := net.Pipe()
	defer application.Close()
	defer endpoint.Close()
	accepted := make(chan error, 1)
	go func() { _, err := io.ReadFull(endpoint, make([]byte, 6)); accepted <- err }()
	applicationStream, err := endpointpkg.OpenApplication(application, applicationResult)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = endpointpkg.Write(endpointResult, endpointpkg.Result{Class: "clean service connection close",
			AuthenticatedTarget: [32]byte{1}, AcceptedBytes: 4096, ReceivedBytes: 4096})
	}()
	result, err := applicationStream.Result()
	if err != nil || result.Class != "clean service connection close" || result.AcceptedBytes != 4096 {
		t.Fatalf("classified result=%+v err=%v", result, err)
	}

	cleanEOF, peer := net.Pipe()
	resultEOF, resultPeer := net.Pipe()
	go func() { _, _ = io.ReadFull(peer, make([]byte, 6)); _ = resultPeer.Close() }()
	defer cleanEOF.Close()
	defer resultEOF.Close()
	stream, err := endpointpkg.OpenApplication(cleanEOF, resultEOF)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Result(); err == nil {
		t.Fatal("clean EOF was treated as semantic Application success")
	}
}
