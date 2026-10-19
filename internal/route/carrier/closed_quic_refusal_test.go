package carrier

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// Valid credentials and occupied TCP/UDP endpoints distinguish profile refusal
// from bind failure, a QUIC exchange, or an attempted TCP fallback.
func TestDeferredQUICRefusesWithoutSocketOrFallback(t *testing.T) {
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	endpoint := tcp.Addr().String()
	udp, err := net.ListenPacket("udp", endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	certificate := entryBindingCertificate(t, 123)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	assertRefused := func(err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("want explicit profile refusal, got %v", err)
		}
	}
	shared, err := ListenClosedSharedCarrier(ClosedCarrierQUIC, endpoint, certificate, func([32]byte) bool { return true }, 1)
	assertRefused(err)
	if shared != nil {
		t.Fatal("QUIC shared listener returned")
	}
	role, err := ListenClosedRoleCarrier(ClosedCarrierQUIC, endpoint, certificate)
	assertRefused(err)
	if role != nil {
		t.Fatal("QUIC role listener returned")
	}
	node, err := OpenClosedNodeCarrier(ctx, ClosedNodeCarrierRequest{CarrierProfile: ClosedCarrierQUIC, Endpoint: endpoint,
		Certificate: certificate, ExpectedPeerKey: identifier(1), Deadline: time.Now().Add(time.Second)})
	assertRefused(err)
	if node != nil {
		t.Fatal("QUIC Node carrier returned")
	}
	if err := tcp.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	connection, err := tcp.Accept()
	if connection != nil {
		_ = connection.Close()
		t.Fatal("QUIC request attempted TCP fallback")
	}
	if failure, ok := err.(net.Error); !ok || !failure.Timeout() {
		t.Fatalf("TCP sentinel did not retain its listener: %v", err)
	}
	if err := udp.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if n, _, err := udp.ReadFrom(make([]byte, 2048)); n != 0 {
		t.Fatal("QUIC request emitted UDP")
	} else if failure, ok := err.(net.Error); !ok || !failure.Timeout() {
		t.Fatalf("UDP sentinel did not retain its socket: %v", err)
	}
}
