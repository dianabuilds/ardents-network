//go:build referencec2

package main

import "testing"

func TestReadConfigRejectsMissingFile(t *testing.T) {
	if _, err := readConfig(t.TempDir() + "/missing.json"); err == nil {
		t.Fatal("missing fixture configuration was accepted")
	}
}

func TestValidAlphaRelayListenAddress(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "127.0.0.1:49100", valid: true},
		{value: "203.0.113.10:49100", valid: true},
		{value: "[::1]:49100", valid: true},
		{value: "relay.example:49100"},
		{value: "127.0.0.1:0"},
		{value: "127.0.0.1:80"},
		{value: "127.0.0.1:not-a-port"},
	} {
		if got := validAlphaRelayListenAddress(test.value); got != test.valid {
			t.Fatalf("validAlphaRelayListenAddress(%q) = %t, want %t", test.value, got, test.valid)
		}
	}
}
