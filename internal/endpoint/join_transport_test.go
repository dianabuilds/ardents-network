//go:build linux

package endpoint

import (
	"net"
	"testing"
)

type retirementWitnessTestConn struct {
	net.Conn
	retired bool
}

func (connection *retirementWitnessTestConn) AuthenticatedPeerRetired() bool {
	return connection.retired
}

func TestJoinedTransportPreservesAuthenticatedPeerRetirement(t *testing.T) {
	local, remote := net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})
	witness := &retirementWitnessTestConn{Conn: local}
	transport := &joinedTransport{Conn: witness}
	if transport.AuthenticatedPeerRetired() {
		t.Fatal("retirement was reported before the Route witness")
	}
	witness.retired = true
	if !transport.AuthenticatedPeerRetired() {
		t.Fatal("joined transport lost the authenticated Route retirement witness")
	}
	transport.Conn = local
	if transport.AuthenticatedPeerRetired() {
		t.Fatal("unwitnessed transport reported authenticated retirement")
	}
}
