//go:build linux

package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

func protectedTLSCredential(t *testing.T) (servicepublication.Credential, ed25519.PrivateKey) {
	t.Helper()
	public, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(signer) })
	var credential servicepublication.Credential
	copy(credential.InstancePublic[:], public)
	credential.Generation = 1
	credential.NotBefore = time.Now().Add(-time.Minute).Unix()
	credential.NotAfter = time.Now().Add(time.Hour).Unix()
	return credential, signer
}

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

func TestProtectedServiceTLSRouteRetirementWitnessSurvivesWrapperChain(t *testing.T) {
	credential, signer := protectedTLSCredential(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	localRaw, remoteRaw := net.Pipe()
	var localRetired, remoteRetired atomic.Bool
	local := &authenticatedRetirementTestConn{Conn: localRaw, retired: &localRetired, peerRetired: &remoteRetired}
	remote := &authenticatedRetirementTestConn{Conn: remoteRaw, retired: &remoteRetired, peerRetired: &localRetired}
	transport := &protectedServiceTransport{Conn: remote}
	clientResult := make(chan *SecuredAttachment, 1)
	clientError := make(chan error, 1)
	go func() {
		secured, _, secureErr := secureProtectedServiceClient(ctx, local, credential, [32]byte{80}, 1)
		clientResult <- secured
		clientError <- secureErr
	}()
	publisherSecured, _, err := secureProtectedServicePublisher(ctx, transport, credential, signer, [32]byte{80}, 1)
	if err != nil {
		t.Fatal(err)
	}
	clientSecured := <-clientResult
	if err := <-clientError; err != nil {
		t.Fatal(err)
	}
	defer publisherSecured.Close()
	closed := make(chan struct{})
	go func() {
		_ = clientSecured.Close()
		close(closed)
	}()
	var one [1]byte
	_, err = (&authenticatedRetirementCarrier{Conn: publisherSecured.connection}).Read(one[:])
	if !errors.Is(err, nativeconnection.ErrAttachmentRetired) {
		t.Fatalf("authenticated retirement through Service wrappers = %v", err)
	}
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("graceful TLS close did not finish")
	}
}

// The peer offers one TLS group while the Service side keeps its selected
// profile. A real Instance signer authenticates each accepted handshake.
func TestProtectedServiceTLSOnlySelectedGroups(t *testing.T) {
	for _, role := range []string{"reader", "publisher"} {
		for _, group := range []tls.CurveID{tls.CurveP256, tls.X25519, tls.X25519MLKEM768} {
			t.Run(role+"/"+group.String(), func(t *testing.T) {
				credential, signer := protectedTLSCredential(t)
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()
				certificate, err := instanceCertificate(credential, signer)
				if err != nil {
					t.Fatal(err)
				}
				local, remote := net.Pipe()
				defer remote.Close()
				peerConfig := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
					CurvePreferences: []tls.CurveID{group}, SessionTicketsDisabled: true}
				var peer *tls.Conn
				result := make(chan error, 1)
				if role == "reader" {
					peerConfig.Certificates = []tls.Certificate{certificate}
					peer = tls.Server(remote, peerConfig)
					go func() {
						secured, _, err := secureProtectedServiceClient(ctx, local, credential, [32]byte{81}, 1)
						if secured != nil {
							_ = secured.Close()
						}
						result <- err
					}()
				} else {
					peerConfig.InsecureSkipVerify = true
					peerConfig.VerifyConnection = verifyInstance(credential.InstancePublic)
					peer = tls.Client(remote, peerConfig)
					go func() {
						secured, _, err := secureProtectedServicePublisher(ctx, local, credential, signer, [32]byte{81}, 1)
						if secured != nil {
							_ = secured.Close()
						}
						result <- err
					}()
				}
				peerErr := peer.HandshakeContext(ctx)
				_ = remote.Close()
				serviceErr := <-result
				if group == tls.CurveP256 {
					if peerErr == nil || serviceErr == nil {
						t.Fatalf("protected Service negotiated unselected P-256: peer=%v service=%v", peerErr, serviceErr)
					}
				} else if peerErr != nil || serviceErr != nil {
					t.Fatalf("selected group refused: peer=%v service=%v", peerErr, serviceErr)
				}
			})
		}
	}
}
