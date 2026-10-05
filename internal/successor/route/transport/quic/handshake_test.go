package quic

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"github.com/quic-go/quic-go"
	"math/big"
	"net"
	"testing"
	"time"
)

// These real certificate/UDP fixtures exercise physical mechanisms only; they
// mint no signed Network observation, token, spend or successful Route result.
func transportCertificate(t *testing.T, start, end time.Time) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: start, NotAfter: end,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var key [32]byte
	copy(key[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, key
}

func transportEndpoint(t *testing.T, _ transport.CarrierProfile) string {
	t.Helper()
	socket, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := socket.LocalAddr().String()
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}
func TestQUICQueuedConnectionRetainsArrivalDeadline(t *testing.T) {
	now := time.Now()
	certificate, key := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
	endpoint := transportEndpoint(t, transport.ClosedCarrierQUIC)
	listener, err := ListenShared(endpoint, certificate, func([32]byte) bool { return false }, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connection, err := quic.DialAddr(ctx, endpoint, transport.RoleClientTLS(key), roleConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseWithError(0, "test-close")
	// A completed connection remains queued while its original interval expires.
	// Making a stream readable before Accept prevents a fresh deadline from
	// accidentally passing this refusal test by timing out on a missing stream.
	time.Sleep(120 * time.Millisecond)
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	accepted, err := listener.Accept(ctx, 60*time.Millisecond)
	if err == nil {
		accepted.Connection.Close()
		t.Fatal("queued connection received a replacement deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) || !transport.IsClosedSharedPeerFailure(err) {
		t.Fatalf("wrong arrival refusal: %v", err)
	}
}
