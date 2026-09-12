//go:build linux

package endpoint

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

// These peers authenticate the actual Instance key but deliberately offer
// a single TLS group. This checks the protected Service configuration, not
// an installed-host or Route/Introduction claim.
func TestTextServiceTLSOnlySelectedGroups(t *testing.T) {
	for _, role := range []string{"reader", "publisher"} {
		for _, group := range []tls.CurveID{tls.CurveP256, tls.X25519, tls.X25519MLKEM768} {
			t.Run(role+"/"+group.String(), func(t *testing.T) {
				client, publisher, _ := textServiceFixture(t)
				binding := client
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()
				lease, err := publisher.owner.endpoint.publications.AcquireAt(ctx, time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				certificate, err := instanceCertificate(publisher.credential, lease)
				if err != nil {
					t.Fatal(err)
				}
				local, remote := net.Pipe()
				defer remote.Close()
				peerConfig := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
					CurvePreferences: []tls.CurveID{group}, SessionTicketsDisabled: true}
				var peer *tls.Conn
				if role == "reader" {
					peerConfig.Certificates = []tls.Certificate{certificate}
					peer = tls.Server(remote, peerConfig)
				} else {
					binding = publisher
					peerConfig.InsecureSkipVerify = true
					peerConfig.VerifyConnection = verifyInstance(publisher.credential.InstancePublic)
					peer = tls.Client(remote, peerConfig)
				}
				done := make(chan error, 1)
				go func() {
					stream, err := binding.openTextServiceStream(ctx, local, fixtureID(81))
					if stream != nil {
						_ = stream.Close()
					}
					done <- err
				}()
				handshakeErr := peer.HandshakeContext(ctx)
				_ = remote.Close()
				<-done
				if group == tls.CurveP256 && handshakeErr == nil {
					t.Fatal("protected Service negotiated unselected P-256")
				}
				if group != tls.CurveP256 && handshakeErr != nil {
					t.Fatalf("selected group refused: %v", handshakeErr)
				}
			})
		}
	}
}
