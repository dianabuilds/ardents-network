package node

import (
	"strings"
	"testing"
)

func TestClosedForwardingUsesTransparentCarrierRelayWithoutChangingStatePeer(t *testing.T) {
	got, err := closedCarrierDialAddress("203.0.113.24:48127", "198.51.100.7:49128")
	if err != nil || got != "198.51.100.7:49128" {
		t.Fatalf("closed Carrier relay = %q / %v", got, err)
	}
	got, err = closedCarrierDialAddress("203.0.113.24:48127", "")
	if err != nil || got != "203.0.113.24:48127" {
		t.Fatalf("closed direct Carrier = %q / %v", got, err)
	}
}

func TestClosedForwardingRejectsInvalidCarrierRelayEndpoint(t *testing.T) {
	for _, input := range []struct{ advertised, relay string }{
		{"hostname.test:48127", "198.51.100.7:49128"},
		{"0.0.0.0:48127", "198.51.100.7:49128"},
		{"203.0.113.24:0", "198.51.100.7:49128"},
		{"203.0.113.24:48127", "relay.test:49128"},
		{"203.0.113.24:48127", "0.0.0.0:49128"},
		{"203.0.113.24:48127", "198.51.100.7:0"},
	} {
		if _, err := closedCarrierDialAddress(input.advertised, input.relay); err == nil || !strings.Contains(err.Error(), "Carrier relay endpoint") {
			t.Fatalf("closed Carrier relay %q -> %q error = %v", input.advertised, input.relay, err)
		}
	}
}
