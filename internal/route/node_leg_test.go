package route

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestOpenNodeLegConfirmsExactTLSAndLegBinding(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 91)
	clientCertificate := entryBindingCertificate(t, 92)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverKey := serverCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	clientKey := clientCertificate.Leaf.PublicKey.(ed25519.PublicKey)
	var serverID, clientID [32]byte
	copy(serverID[:], serverKey)
	copy(clientID[:], clientKey)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: identifier(3), Epoch: 4,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: clientID, PeerNodeID: serverID, NotAfter: deadline}
	serverDone := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		secured := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert,
			SessionTicketsDisabled: true, NextProtos: []string{Profile}})
		if acceptErr = secured.HandshakeContext(context.Background()); acceptErr == nil {
			peer := binding
			peer.SenderRole, peer.PeerRole = binding.PeerRole, binding.SenderRole
			peer.SenderNodeID, peer.PeerNodeID = binding.PeerNodeID, binding.SenderNodeID
			acceptErr = AcceptNodeLegBinding(secured, peer)
		}
		serverDone <- secured.Close()
		if acceptErr != nil {
			serverDone <- acceptErr
		}
	}()
	connection, err := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierTCP, Endpoint: listener.Addr().String(), Certificate: clientCertificate,
		ExpectedPeerKey: identifierFromKey(serverKey), Binding: binding, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if _, readErr := connection.Read(make([]byte, 1)); readErr == nil {
		t.Fatal("closed Carrier remained readable")
	} else if class, ok := CarrierFailureClassOf(readErr); !ok || class != CarrierFailureClosed {
		t.Fatalf("closed Carrier classification = %q, %v", class, readErr)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestOpenNodeLegRejectsUnknownCarrierWithoutDial(t *testing.T) {
	certificate := entryBindingCertificate(t, 95)
	key := certificate.Leaf.PublicKey.(ed25519.PublicKey)
	nodeID := identifierFromKey(key)
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(21), Digest: identifier(22), AttachmentID: identifier(23), Epoch: 24,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: nodeID, PeerNodeID: identifier(25), NotAfter: deadline}
	carrier, openErr := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierProfile("unknown"), Endpoint: "127.0.0.1:9",
		Certificate: certificate, ExpectedPeerKey: identifier(26), Binding: binding, Deadline: deadline})
	if openErr == nil || carrier != nil {
		t.Fatal("unknown Carrier Profile reached a transport adapter")
	}
	if class, ok := CarrierFailureClassOf(openErr); !ok || class != CarrierFailureIncompatible {
		t.Fatalf("unknown Carrier Profile classification = %q, %v", class, openErr)
	}
}

func TestFailedTCPAttemptNeverFallsBackToListeningQUICSocket(t *testing.T) {
	packet, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer packet.Close()
	if err := packet.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	received := make(chan bool, 1)
	go func() {
		buffer := make([]byte, 1)
		_, _, readErr := packet.ReadFrom(buffer)
		received <- readErr == nil
	}()
	clientCertificate := entryBindingCertificate(t, 99)
	serverCertificate := entryBindingCertificate(t, 100)
	clientID := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	serverID := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	deadline := time.Now().UTC().Add(time.Second).Truncate(time.Second)
	binding := LegBinding{NetworkID: identifier(41), Digest: identifier(42), AttachmentID: identifier(43), Epoch: 44,
		SenderRole: InitiatorRole, PeerRole: RendezvousRole, SenderNodeID: clientID, PeerNodeID: serverID, NotAfter: deadline}
	if carrier, err := OpenNodeLeg(t.Context(), NodeLegRequest{CarrierProfile: CarrierTCP, Endpoint: packet.LocalAddr().String(),
		Certificate: clientCertificate, ExpectedPeerKey: serverID, Binding: binding, Deadline: deadline}); err == nil || carrier != nil {
		t.Fatal("failed TCP attempt returned a Carrier")
	}
	if <-received {
		t.Fatal("TCP failure contacted the UDP socket as a fallback")
	}
}

func identifierFromKey(key ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], key)
	return result
}
