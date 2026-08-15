package main

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
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

func TestStreamCountsPermitBoundedSustainedStage4Workload(t *testing.T) {
	send, receive, err := streamCounts("268435456", "0")
	if err != nil || send != 256<<20 || receive != 0 {
		t.Fatalf("sustained stream counts=%d/%d err=%v", send, receive, err)
	}
	if _, _, err := streamCounts("268435457", "0"); err == nil {
		t.Fatal("stream count above the Stage 4 bound was accepted")
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
