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
	connection, err := OpenNodeLeg(t.Context(), NodeLegRequest{Endpoint: listener.Addr().String(), Certificate: clientCertificate,
		ExpectedPeerKey: identifierFromKey(serverKey), Binding: binding, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func identifierFromKey(key ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], key)
	return result
}
