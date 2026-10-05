package transport_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestLiteralEndpointRefusesAmbientNamesAndInvalidPorts(t *testing.T) {
	for _, endpoint := range []string{"127.0.0.1:443", "192.0.2.1:1", "[2001:db8::1]:65535", "[::ffff:192.0.2.1]:443"} {
		if !transport.LiteralEndpoint(endpoint) {
			t.Fatalf("literal endpoint refused: %q", endpoint)
		}
	}
	for _, endpoint := range []string{"", "localhost:443", "node.example:443", "https://192.0.2.1:443", "192.0.2.1", "192.0.2.1:0", "192.0.2.1:65536", "192.0.2.1:-1", "192.0.2.1:https", "[fe80::1%eth0]:443", "2001:db8::1:443"} {
		if transport.LiteralEndpoint(endpoint) {
			t.Fatalf("ambient or invalid endpoint accepted: %q", endpoint)
		}
	}
}

func TestClosedTransportIdentitiesRetainAcceptedBytes(t *testing.T) {
	if string(transport.ClosedCarrierTCP) != "ardents-carrier-tcp-tls-v2" ||
		string(transport.ClosedCarrierQUIC) != "ardents-carrier-quic-v2" ||
		transport.ClosedRouteProfile != "ardents-route-v3" {
		t.Fatal("accepted Carrier or ALPN identity changed")
	}
}
