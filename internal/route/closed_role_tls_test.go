//go:build linux

package route

import (
	"crypto/ed25519"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestClosedRoleTLSAuthenticatesOnlyServerAndForbidsClientCertificate(t *testing.T) {
	serverCertificate := entryBindingCertificate(t, 131)
	serverID := identifierFromKey(serverCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	serverRaw, clientRaw := net.Pipe()
	defer serverRaw.Close()
	defer clientRaw.Close()
	deadline := time.Now().Add(10 * time.Second)
	serverDone := make(chan error, 1)
	go func() {
		_, err := AcceptClosedRoleTLS(t.Context(), serverRaw, serverCertificate, deadline)
		serverDone <- err
	}()
	_, err := OpenClosedRoleTLS(t.Context(), clientRaw, serverID, deadline)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	config := closedRoleClientTLS(serverID)
	serverConfig := closedRoleServerTLS(entryBindingCertificate(t, 132))
	if len(config.Certificates) != 0 || config.ClientSessionCache != nil || !config.SessionTicketsDisabled || serverConfig.ClientAuth != tls.NoClientCert {
		t.Fatal("closed role TLS enabled client identity or resumption")
	}
}
