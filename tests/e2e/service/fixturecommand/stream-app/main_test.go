package main

import (
	"testing"
	"time"
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

func TestStreamCountsPermitBoundedSustainedWorkload(t *testing.T) {
	send, receive, err := streamCounts("268435456", "0")
	if err != nil || send != 256<<20 || receive != 0 {
		t.Fatalf("sustained stream counts=%d/%d err=%v", send, receive, err)
	}
	if _, _, err := streamCounts("268435457", "0"); err == nil {
		t.Fatal("stream count above the product bound was accepted")
	}
}
