package main

import (
	"net"
	"testing"
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
	go func() { value, err := exchange(client, "client", 17, 91, 4096); results <- outcome{value, err} }()
	go func() { value, err := exchange(publisher, "publisher", 91, 17, 4096); results <- outcome{value, err} }()
	for range 2 {
		result := <-results
		if result.err != nil || result.value.Terminal != "success" || result.value.SentBytes != 4096 || result.value.ReceivedBytes != 4096 {
			t.Fatalf("opaque external Application failed: value=%+v err=%v", result.value, result.err)
		}
	}
}
