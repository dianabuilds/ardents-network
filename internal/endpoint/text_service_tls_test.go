//go:build linux

package endpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

type authenticatedRetirementTestConn struct {
	net.Conn
	retired, peerRetired *atomic.Bool
}

func (connection *authenticatedRetirementTestConn) Close() error {
	connection.peerRetired.Store(true)
	return connection.Conn.Close()
}

func (connection *authenticatedRetirementTestConn) AuthenticatedPeerRetired() bool {
	return connection.retired.Load()
}

func TestTextServiceTLSRouteRetirementWitnessSurvivesWrapperChain(t *testing.T) {
	reader, publisher, _ := textServiceFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	lease, err := publisher.owner.endpoint.publications.AcquireAt(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	localRaw, remoteRaw := net.Pipe()
	var localRetired, remoteRetired atomic.Bool
	local := &authenticatedRetirementTestConn{Conn: localRaw, retired: &localRetired, peerRetired: &remoteRetired}
	remote := &authenticatedRetirementTestConn{Conn: remoteRaw, retired: &remoteRetired, peerRetired: &localRetired}
	joined := &textJoinedTransport{Conn: remote, stop: func() {}, finish: func(err error) error { return err }}
	service := &textServiceTransport{Conn: joined}
	clientResult := make(chan *securedAttachment, 1)
	clientError := make(chan error, 1)
	go func() {
		secured, _, secureErr := secureTextClient(ctx, local, reader.credential, fixtureID(80), 1)
		clientResult <- secured
		clientError <- secureErr
	}()
	publisherSecured, _, err := secureTextPublisher(ctx, service, publisher.credential, lease, fixtureID(80), 1)
	if err != nil {
		t.Fatal(err)
	}
	clientSecured := <-clientResult
	if err := <-clientError; err != nil {
		t.Fatal(err)
	}
	defer publisherSecured.close()
	closed := make(chan struct{})
	go func() {
		clientSecured.close()
		close(closed)
	}()
	var one [1]byte
	_, err = (&authenticatedRetirementCarrier{Conn: publisherSecured.connection}).Read(one[:])
	if !errors.Is(err, nativeconnection.ErrAttachmentRetired) {
		t.Fatalf("Route retirement through Endpoint wrappers = %v", err)
	}
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("graceful TLS close did not finish")
	}
}

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
					stream, err := binding.openTextServiceStreamWithRecovery(ctx, local, fixtureID(81), nil)
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
