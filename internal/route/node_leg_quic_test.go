package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestOpenNodeLegUsesSameBindingOverQUIC(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 93)
	clientCertificate := entryBindingCertificate(t, 94)
	serverKey := serverCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	clientKey := clientCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	serverID, clientID := identifierFromKey(serverKey), identifierFromKey(clientKey)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(11), Digest: identifier(12), AttachmentID: identifier(13), Epoch: 14,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: clientID, PeerNodeID: serverID, NotAfter: deadline}
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert,
		SessionTicketsDisabled: true, NextProtos: []string{Profile}}, testQUICConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept(context.Background())
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		stream, acceptErr := connection.AcceptStream(context.Background())
		if acceptErr == nil {
			peer := binding
			peer.SenderRole, peer.PeerRole = binding.PeerRole, binding.SenderRole
			peer.SenderNodeID, peer.PeerNodeID = binding.PeerNodeID, binding.SenderNodeID
			acceptErr = AcceptNodeLegBinding(stream, peer)
		}
		if acceptErr == nil {
			buffer := make([]byte, 1)
			if _, acceptErr = io.ReadFull(stream, buffer); acceptErr == nil {
				_, acceptErr = stream.Write(buffer)
			}
		}
		serverDone <- acceptErr
	}()
	carrier, err := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierQUIC, Endpoint: listener.Addr().String(),
		Certificate: clientCertificate, ExpectedPeerKey: serverID, Binding: binding, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(6 * time.Second)
	if _, err := carrier.Write([]byte{0x42}); err != nil {
		t.Fatalf("idle QUIC Carrier write failed: %v", err)
	}
	response := make([]byte, 1)
	if _, err := io.ReadFull(carrier, response); err != nil || response[0] != 0x42 {
		t.Fatalf("idle QUIC Carrier response = %x, %v", response, err)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestQUICCarrierProfileKeepsOptionalSemanticsDisabled(t *testing.T) {
	profile := nodeQUICConfig()
	if profile.Allow0RTT || profile.EnableDatagrams || profile.InitialPacketSize != 1200 ||
		profile.MaxIncomingStreams != -1 || profile.MaxIncomingUniStreams != -1 ||
		profile.KeepAlivePeriod <= 0 || profile.KeepAlivePeriod >= profile.MaxIdleTimeout {
		t.Fatal("QUIC Carrier Profile optional or idle semantics are invalid")
	}
}

func TestFailedQUICAttemptNeverFallsBackToListeningTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	accepted := make(chan bool, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
			accepted <- true
			return
		}
		accepted <- false
	}()
	clientCertificate := entryBindingCertificate(t, 96)
	serverCertificate := entryBindingCertificate(t, 97)
	clientID := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	serverID := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	deadline := time.Now().UTC().Add(time.Second).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(31), Digest: identifier(32), AttachmentID: identifier(33), Epoch: 34,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: clientID, PeerNodeID: serverID, NotAfter: deadline}
	carrier, openErr := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierQUIC, Endpoint: listener.Addr().String(),
		Certificate: clientCertificate, ExpectedPeerKey: serverID, Binding: binding, Deadline: deadline})
	if openErr == nil || carrier != nil {
		t.Fatal("failed QUIC attempt returned a Carrier")
	}
	if class, ok := CarrierFailureClassOf(openErr); !ok || class != CarrierFailureTimeout {
		t.Fatalf("failed QUIC attempt classification = %q, %v", class, openErr)
	}
	if <-accepted {
		t.Fatal("QUIC failure contacted the TCP listener as a fallback")
	}
}

func TestFailedQUICBindingAbortsInsteadOfGracefullyClosing(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 101)
	clientCertificate := entryBindingCertificate(t, 102)
	serverID := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	clientID := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	deadline := time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(51), Digest: identifier(52), AttachmentID: identifier(53), Epoch: 54,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: clientID, PeerNodeID: serverID, NotAfter: deadline}
	listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert,
		SessionTicketsDisabled: true, NextProtos: []string{Profile}}, testQUICConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept(context.Background())
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		stream, acceptErr := connection.AcceptStream(context.Background())
		if acceptErr == nil {
			_, acceptErr = ReadNodeLegBinding(stream)
		}
		if acceptErr == nil {
			wrong := binding
			wrong.SenderRole, wrong.PeerRole = binding.PeerRole, binding.SenderRole
			wrong.SenderNodeID, wrong.PeerNodeID = binding.PeerNodeID, identifier(55)
			acceptErr = WriteNodeLegBinding(stream, wrong)
		}
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		<-connection.Context().Done()
		serverDone <- context.Cause(connection.Context())
	}()
	carrier, openErr := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierQUIC, Endpoint: listener.Addr().String(),
		Certificate: clientCertificate, ExpectedPeerKey: serverID, Binding: binding, Deadline: deadline})
	if openErr == nil || carrier != nil {
		t.Fatal("invalid reciprocal binding returned a Carrier")
	}
	if class, ok := CarrierFailureClassOf(openErr); !ok || class != CarrierFailureUnauthorized {
		t.Fatalf("invalid reciprocal binding classification = %q, %v", class, openErr)
	}
	remoteErr := <-serverDone
	var applicationErr *quic.ApplicationError
	if !errors.As(remoteErr, &applicationErr) || applicationErr.ErrorCode == 0 {
		t.Fatalf("failed QUIC binding closed without an abort code: %v", remoteErr)
	}
}

func testQUICConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, MaxIncomingStreams: 1, MaxIncomingUniStreams: -1,
		InitialPacketSize: 1200, InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false }, EnableDatagrams: false, Allow0RTT: false}
}
