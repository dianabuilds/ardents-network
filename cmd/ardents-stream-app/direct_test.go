package main

import (
	"bytes"
	"net"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/streamworkload"
)

func TestDirectTransferUsesTheSameBoundedSeededStream(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	seed := [32]byte{33}
	type outcome struct {
		value streamworkload.Observation
		err   error
	}
	results := make(chan outcome, 2)
	go func() {
		value, err := streamworkload.Exchange(left, "direct-connect", seed, seed, 1<<20, 0, nil, nil)
		results <- outcome{value, err}
	}()
	go func() {
		value, err := streamworkload.Exchange(right, "direct-listen", seed, seed, 0, 1<<20, nil, nil)
		results <- outcome{value, err}
	}()
	for range 2 {
		result := <-results
		if result.err != nil || result.value.Terminal != "success" {
			t.Fatalf("direct transfer=%+v err=%v", result.value, result.err)
		}
	}
}

func TestDirectModeRejectsIncompleteArguments(t *testing.T) {
	if err := runDirect([]string{"direct", "listen"}, &bytes.Buffer{}); err == nil {
		t.Fatal("incomplete direct mode was accepted")
	}
}
