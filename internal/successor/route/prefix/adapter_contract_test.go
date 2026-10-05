package prefix_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	rolequic "github.com/dianabuilds/ardents-network/internal/successor/route/transport/quic"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

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

func transportEndpoint(t *testing.T, profile transport.CarrierProfile) string {
	t.Helper()
	if profile == transport.ClosedCarrierQUIC {
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func TestActualSharedTransportClassification(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, node := range []bool{false, true} {
			t.Run(string(profile)+map[bool]string{false: "/direct", true: "/node"}[node], func(t *testing.T) {
				now := time.Now()
				serverCert, serverKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
				clientCert, clientKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
				endpoint := transportEndpoint(t, profile)
				listener, err := listenTestSharedCarrier(profile, endpoint, serverCert, func(key [32]byte) bool { return key == clientKey }, 1)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				result := make(chan error, 1)
				go func() {
					accepted, err := listener.Accept(ctx, time.Second)
					if err != nil {
						result <- err
						return
					}
					defer accepted.Connection.Close()
					want := transport.ClosedSharedDirect
					if node {
						want = transport.ClosedSharedNode
					}
					if accepted.Kind != want || node && accepted.NodeKey != clientKey {
						result <- errors.New("wrong shared classification")
						return
					}
					value := make([]byte, 3)
					if _, err := io.ReadFull(accepted.Connection, value); err != nil {
						result <- err
						return
					}
					if string(value) != "one" {
						result <- errors.New("wrong transport bytes")
						return
					}
					if _, err = accepted.Connection.Write([]byte("two")); err != nil {
						result <- err
						return
					}
					// Keep physical QUIC retirement after peer consumption. Closing
					// a connection aborts outstanding traffic, and is no flush proof.
					_, err = io.ReadFull(accepted.Connection, value)
					if err == nil && string(value) != "ack" {
						err = errors.New("wrong consumption acknowledgement")
					}
					result <- err
				}()
				var client transport.Carrier
				if node {
					client, err = openTestNodeCarrier(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, Certificate: clientCert, ExpectedPeerKey: serverKey, Deadline: time.Now().Add(time.Second)})
				} else {
					client, err = openTestRoleCarrier(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, ExpectedServer: serverKey, Deadline: time.Now().Add(time.Second)})
				}
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				if err := client.SetDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				if _, err := client.Write([]byte("one")); err != nil {
					t.Fatal(err)
				}
				value := make([]byte, 3)
				if _, err := io.ReadFull(client, value); err != nil {
					t.Fatal(err)
				}
				if string(value) != "two" {
					t.Fatal("wrong response bytes")
				}
				if _, err := client.Write([]byte("ack")); err != nil {
					t.Fatal(err)
				}
				if err := <-result; err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestActualTransportRejectsServerIdentityAndDates(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, refusal := range []string{"wrong-key", "expired", "future"} {
			t.Run(string(profile)+"/"+refusal, func(t *testing.T) {
				now := time.Now()
				start, end := now.Add(-time.Hour), now.Add(time.Hour)
				if refusal == "expired" {
					end = now.Add(-time.Minute)
				}
				if refusal == "future" {
					start = now.Add(time.Minute)
				}
				certificate, key := transportCertificate(t, start, end)
				if refusal == "wrong-key" {
					key[0] ^= 1
				}
				endpoint := transportEndpoint(t, profile)
				listener, err := listenTestSharedCarrier(profile, endpoint, certificate, func([32]byte) bool { return false }, 1)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				joined := make(chan struct{})
				go func() {
					defer close(joined)
					accepted, err := listener.Accept(ctx, time.Second)
					if err == nil {
						accepted.Connection.Close()
					}
				}()
				client, err := openTestRoleCarrier(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: endpoint, ExpectedServer: key, Deadline: time.Now().Add(time.Second)})
				if err == nil {
					client.Close()
					t.Error("invalid server accepted")
				}
				cancel()
				listener.Close()
				<-joined
			})
		}
	}
}

// openTestRoleCarrier selects exactly the authenticated profile's actual adapter, without fallback.
func openTestRoleCarrier(ctx context.Context, input transport.ClosedRoleCarrierRequest) (net.Conn, error) {
	if err := transport.ValidateRoleRequest(ctx, input); err != nil {
		return nil, err
	}
	switch input.CarrierProfile {
	case transport.ClosedCarrierTCP:
		return roletls.OpenEndpoint(ctx, input)
	case transport.ClosedCarrierQUIC:
		return rolequic.OpenEndpoint(ctx, input)
	default:
		return nil, errors.New("closed role carrier profile is unsupported")
	}
}

// openTestNodeCarrier selects exactly the authenticated profile's actual adapter, without fallback.
func openTestNodeCarrier(ctx context.Context, input transport.ClosedNodeCarrierRequest) (transport.Carrier, error) {
	if err := transport.ValidateNodeRequest(ctx, input); err != nil {
		return nil, err
	}
	switch input.CarrierProfile {
	case transport.ClosedCarrierTCP:
		return roletls.OpenNode(ctx, input)
	case transport.ClosedCarrierQUIC:
		return rolequic.OpenNode(ctx, input)
	default:
		return nil, errors.New("closed Node Carrier profile is unsupported")
	}
}

// listenTestSharedCarrier selects the actual shared listener without fallback.
func listenTestSharedCarrier(profile transport.CarrierProfile, endpoint string, certificate tls.Certificate, verify transport.ClosedSharedPeerVerifier, handshakeLimit uint16) (transport.ClosedSharedCarrierListener, error) {
	if err := transport.ValidateSharedListener(endpoint, certificate, verify, handshakeLimit); err != nil {
		return nil, err
	}
	switch profile {
	case transport.ClosedCarrierTCP:
		return roletls.ListenShared(endpoint, certificate, verify, handshakeLimit)
	case transport.ClosedCarrierQUIC:
		return rolequic.ListenShared(endpoint, certificate, verify, handshakeLimit)
	default:
		return nil, errors.New("closed shared carrier profile is unsupported")
	}
}
