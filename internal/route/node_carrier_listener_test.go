package route

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestQUICListenerReturnsPendingCarrierBeforeAuthenticationCompletes(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 98)
	clientCertificate := entryBindingCertificate(t, 99)
	endpoint := availableUDPNodeCarrierEndpoint(t)
	listener, err := ListenNodeCarrier(CarrierQUIC, endpoint, serverCertificate)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	verificationReached := make(chan struct{})
	releaseVerification := make(chan struct{})
	clientDone := make(chan error, 1)
	go func() {
		connection, dialErr := quic.DialAddr(context.Background(), endpoint, &tls.Config{
			MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{clientCertificate},
			InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{Profile},
			VerifyConnection: func(tls.ConnectionState) error {
				close(verificationReached)
				<-releaseVerification
				return errors.New("test stops the client handshake")
			}}, nodeQUICConfig())
		if connection != nil {
			_ = connection.CloseWithError(1, "unexpected-authentication")
		}
		clientDone <- dialErr
	}()
	select {
	case <-verificationReached:
	case <-time.After(time.Second):
		t.Fatal("QUIC client did not reach its blocked verification point")
	}
	acceptContext, cancelAccept := context.WithTimeout(t.Context(), time.Second)
	defer cancelAccept()
	pending, err := listener.Accept(acceptContext)
	if err != nil {
		t.Fatal(err)
	}
	close(releaseVerification)
	if err := <-clientDone; err == nil {
		t.Fatal("blocked client authentication unexpectedly succeeded")
	}
	authContext, cancelAuth := context.WithTimeout(t.Context(), time.Second)
	defer cancelAuth()
	if carrier, _, err := pending.Authenticate(authContext, time.Now().Add(time.Second)); err == nil || carrier != nil {
		t.Fatal("failed QUIC authentication returned a Carrier")
	}
	_ = pending.Close()
}

func availableUDPNodeCarrierEndpoint(t *testing.T) string {
	t.Helper()
	connection, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := connection.LocalAddr().String()
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}
