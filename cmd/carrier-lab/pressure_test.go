package main

import "testing"

func TestPressureMemoryRejectsUnboundedRequests(t *testing.T) {
	for _, arguments := range [][]string{{}, {"-bytes", "1", "-duration", "1s"},
		{"-bytes", "67108864", "-duration", "2m"},
		{"-bytes", "67108864", "-duration", "1s", "-connect", "invalid"}} {
		if pressureMemory(arguments) != 64 {
			t.Fatalf("unbounded pressure request accepted: %v", arguments)
		}
	}
}
