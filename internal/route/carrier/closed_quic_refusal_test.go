package carrier

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// Valid credentials and occupied TCP/UDP endpoints distinguish profile refusal
// from bind failure, a QUIC exchange, or an attempted TCP fallback.
func TestDeferredQUICRefusesWithoutSocketOrFallback(t *testing.T) {
	t.Run("occupied-udp-candidate", func(t *testing.T) {
		first, occupiedUDP := carrierRefusalSentinels(t, nil)
		defer first.Close()
		defer occupiedUDP.Close()
		tcp, udp := carrierRefusalSentinels(t, first)
		defer tcp.Close()
		defer udp.Close()
		if tcp.Addr().String() == occupiedUDP.LocalAddr().String() || tcp.Addr().String() != udp.LocalAddr().String() {
			t.Fatal("fixture did not acquire its own jointly bindable endpoint")
		}
		if err := first.(*net.TCPListener).SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
			t.Fatal("unqualified candidate retained its TCP listener", err)
		}
	})
	tcp, udp := carrierRefusalSentinels(t, nil)
	defer tcp.Close()
	endpoint := tcp.Addr().String()
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

// TCP and UDP have separate reservations: a TCP ephemeral port can be excluded
// for UDP. Qualify the fixture only after both sentinels are owned. No carrier
// assertion or operation runs while selecting these bounded local resources.
func carrierRefusalSentinels(t *testing.T, first net.Listener) (net.Listener, net.PacketConn) {
	t.Helper()
	var last error
	for candidate := range 8 {
		tcp := first
		first = nil
		var err error
		if tcp == nil {
			tcp, err = net.Listen("tcp", "127.0.0.1:0")
		}
		if err != nil {
			t.Fatal("invalid environment: TCP probe listener", err)
		}
		udp, err := net.ListenPacket("udp", tcp.Addr().String())
		if err == nil {
			return tcp, udp
		}
		last = err
		t.Log("unqualified TCP/UDP probe candidate", candidate+1, err)
		if err := tcp.Close(); err != nil {
			t.Fatal("original unqualified probe did not close", err)
		}
	}
	t.Fatal("invalid environment: no jointly bindable TCP/UDP probe endpoint", last)
	return nil, nil
}
