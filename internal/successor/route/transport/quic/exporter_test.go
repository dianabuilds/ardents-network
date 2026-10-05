package quic

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestRoleExporterIsAvailableOnlyToActualRolePrincipal(t *testing.T) {
	for _, node := range []bool{false, true} {
		name := "role"
		if node {
			name = "node"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			serverCertificate, serverKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			clientCertificate, clientKey := transportCertificate(t, now.Add(-time.Hour), now.Add(time.Hour))
			endpoint := transportEndpoint(t, transport.ClosedCarrierQUIC)
			listener, err := ListenShared(endpoint, serverCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			end, _ := ctx.Deadline()
			type acceptance struct {
				value transport.ClosedSharedCarrier
				err   error
			}
			incoming := make(chan acceptance, 1)
			go func() { value, err := listener.Accept(ctx, time.Second); incoming <- acceptance{value, err} }()
			var client net.Conn
			if node {
				var value transport.Carrier
				value, err = OpenNode(ctx, transport.ClosedNodeCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC,
					Endpoint: endpoint, Certificate: clientCertificate, ExpectedPeerKey: serverKey, Deadline: end})
				if err == nil {
					client = value.(net.Conn)
				}
			} else {
				client, err = OpenEndpoint(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierQUIC,
					Endpoint: endpoint, ExpectedServer: serverKey, Deadline: end})
			}
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			written := make(chan error, 1)
			go func() { _, err := client.Write([]byte{1}); written <- err }()
			peer := <-incoming
			if peer.err != nil {
				t.Fatal(peer.err)
			}
			defer peer.value.Connection.Close()
			if err := peer.value.Connection.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			var value [1]byte
			if _, err := io.ReadFull(peer.value.Connection, value[:]); err != nil || value[0] != 1 {
				t.Fatal("actual stream input failed", err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			clientExporter, clientErr := transport.ClosedRoleTLSExporter(client)
			peerExporter, peerErr := transport.ClosedRoleTLSExporter(peer.value.Connection)
			if node {
				if clientExporter != nil || peerExporter != nil || clientErr == nil || peerErr == nil {
					t.Fatal("Node principal acquired a role exporter")
				}
				if _, ok := peer.value.Connection.(interface {
					RoleTLSExporter() (transport.ClosedTLSExporter, error)
				}); ok {
					t.Fatal("receiving Node exposed the role capability")
				}
				return
			}
			if clientErr != nil || peerErr != nil {
				t.Fatal("authenticated role exporter absent", clientErr, peerErr)
			}
			left, err := clientExporter("EXPORTER-route-adapter-test", nil, 32)
			if err != nil {
				t.Fatal(err)
			}
			right, err := peerExporter("EXPORTER-route-adapter-test", nil, 32)
			if err != nil || !bytes.Equal(left, right) {
				t.Fatal("actual role exporter transcripts differ", err)
			}
		})
	}
}
