package node

import (
	"strings"
	"testing"
)

func TestClosedNodeUsesPrivateLocalRelayTargetWithoutChangingAdvertisedEndpoint(t *testing.T) {
	listen, err := closedListenAddress("203.0.113.24:48127", "172.17.0.1:49127")
	if err != nil || listen != "172.17.0.1:49127" {
		t.Fatalf("closed relay target = %q / %v", listen, err)
	}
	advertised, err := closedListenAddress("203.0.113.24:48127", "")
	if err != nil || advertised != "203.0.113.24:48127" {
		t.Fatalf("closed advertised bind = %q / %v", advertised, err)
	}
}

func TestClosedNodeRejectsNonPrivateRelayTarget(t *testing.T) {
	for _, input := range []struct{ advertised, override string }{
		{"hostname.test:48127", "172.17.0.1:49127"},
		{"0.0.0.0:48127", "172.17.0.1:49127"},
		{"203.0.113.24:0", "172.17.0.1:49127"},
		{"203.0.113.24:48127", "localhost:49127"},
		{"203.0.113.24:48127", "0.0.0.0:49127"},
		{"203.0.113.24:48127", "198.51.100.7:49127"},
		{"203.0.113.24:48127", "127.0.0.1:0"},
	} {
		if _, err := closedListenAddress(input.advertised, input.override); err == nil || !strings.Contains(err.Error(), "private listen override") {
			t.Fatalf("closed override %q -> %q error = %v", input.advertised, input.override, err)
		}
	}
}
