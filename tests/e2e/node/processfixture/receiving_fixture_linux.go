//go:build linux && node_process_native

package processfixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	"testing"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// AddReceivingFixtureNode adds a signed receiving duty and reserves its test port.
func AddReceivingFixtureNode(t testing.TB, spec *networkfixture.ClosedSpec, listeners map[[32]byte]net.Listener, certificates map[[32]byte]tls.Certificate, carrier string, index, domain, subrole byte) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(key) })
	id := [32]byte{index}
	listeners[id], certificates[id] = listener, RoleCertificate(t, key)
	spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{
		RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter,
			Endpoint: listener.Addr().String(), Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key},
		RoleDomain: domain, Subrole: subrole,
	})
}
