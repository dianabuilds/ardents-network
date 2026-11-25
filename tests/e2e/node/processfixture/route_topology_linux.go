//go:build linux

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

// AddPublicationFixtureNodes adds signed topology inputs and reserves their ports until receivers open.
func AddPublicationFixtureNodes(t testing.TB, spec *networkfixture.ClosedSpec, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate,
	bindHost, carrier string, tcp, alternateForwarders bool) {
	t.Helper()
	ids := [][32]byte{{17}, {21}, {22}, {23}, {24}, {25}, {31}, {32}, {33}, {34}, {41}}
	if alternateForwarders {
		ids = append(ids, [32]byte{18}, [32]byte{19})
	}
	for _, id := range ids {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		address, release := reserveTopologyAddress(t, bindHost, tcp)
		reservations[id] = release
		domain, subrole := byte(4), byte(1)
		if id[0] >= 23 {
			subrole = 2
		}
		if id == [32]byte{25} {
			subrole = 3
		}
		if id == [32]byte{17} {
			domain, subrole = 2, 5
		}
		if id == [32]byte{18} || id == [32]byte{19} {
			domain, subrole = 1, 2
		}
		if id[0] >= 31 {
			domain, subrole = 3, 1
			if id[0] >= 33 {
				subrole = 2
			}
		}
		if id == [32]byte{41} {
			domain, subrole = 2, 4
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter, Endpoint: address, Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole})
		certificates[id] = RoleCertificate(t, key)
	}
}

// AddRouteFixtureNodes prepares the base role topology and optional network Issuer.
func AddRouteFixtureNodes(t testing.TB, spec *networkfixture.ClosedSpec, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate,
	bindHost, carrier string, tcp bool, domain byte, introduction, issuerNetwork bool) {
	t.Helper()
	count := 4
	if introduction {
		count++
	}
	for i := 0; i < count; i++ {
		id := [32]byte{byte(12 + i)}
		var key ed25519.PrivateKey
		if i == 0 {
			key = spec.Nodes[1].PrivateKey
		} else {
			var err error
			_, key, err = ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { clear(key) })
		}
		address, release := reserveTopologyAddress(t, bindHost, tcp)
		reservations[id] = release
		subrole := byte(1)
		if i >= 2 {
			subrole = 2
		}
		if i == 4 {
			subrole = 3
		}
		node := networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter, Endpoint: address, Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole}
		if i == 0 {
			spec.Nodes[1] = node
		} else {
			spec.Nodes = append(spec.Nodes, node)
		}
		certificates[id] = RoleCertificate(t, key)
	}
	if issuerNetwork {
		node := &spec.Nodes[0]
		address, release := reserveTopologyAddress(t, bindHost, tcp)
		reservations[node.NodeID] = release
		node.Endpoint, node.Carrier = address, carrier
		certificates[node.NodeID] = RoleCertificate(t, node.PrivateKey)
	}
}

func reserveTopologyAddress(t testing.TB, bindHost string, tcp bool) (string, func()) {
	t.Helper()
	address := net.JoinHostPort(bindHost, "0")
	var bound net.Addr
	var closeSocket func() error
	if tcp {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		bound, closeSocket = listener.Addr(), listener.Close
	} else {
		socket, err := net.ListenPacket("udp", address)
		if err != nil {
			t.Fatal(err)
		}
		bound, closeSocket = socket.LocalAddr(), socket.Close
	}
	release := func() {
		if err := closeSocket(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("release reserved fixture address %s: %v", bound, err)
		}
	}
	t.Cleanup(release)
	return bound.String(), release
}

// AddDescriptorFixtureNodes adds the live Descriptor and signed Introduction duties.
func AddDescriptorFixtureNodes(t testing.TB, spec *networkfixture.ClosedSpec, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate, carrier string, tcp bool) {
	t.Helper()
	for _, id := range [][32]byte{{17}, {18}} {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		address := "127.0.0.1:1"
		if id == [32]byte{17} {
			bound, release := reserveTopologyAddress(t, "127.0.0.1", tcp)
			address, reservations[id] = bound, release
		}
		domain, subrole := byte(2), byte(5)
		if id == [32]byte{18} {
			domain, subrole = 4, 3
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter, Endpoint: address, Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole})
		certificates[id] = RoleCertificate(t, key)
	}
}

// AddJoinFixtureNodes binds independent Source, Responder and JOIN roles in the signed fixture.
func AddJoinFixtureNodes(t testing.TB, spec *networkfixture.ClosedSpec, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate, carrier string, tcp bool) {
	t.Helper()
	// Keep the independently signed issuer (11), replace the initial receiver
	// with Source Entry (12), and add the remaining opposite-role duties.
	spec.Nodes = spec.Nodes[:1]
	for idByte := byte(12); idByte <= 21; idByte++ {
		id := [32]byte{idByte}
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		address, release := reserveTopologyAddress(t, "127.0.0.1", tcp)
		reservations[id] = release
		domain, subrole := byte(1), byte(1)
		switch {
		case idByte < 16:
			if idByte >= 14 {
				subrole = 2
			}
		case idByte < 20:
			domain = 3
			if idByte >= 18 {
				subrole = 2
			}
		default:
			domain = 2
			subrole = 4
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore, ValidUntil: spec.NotAfter, Endpoint: address, Carrier: carrier, Capability: 2, Capacity: 1, PrivateKey: key}, RoleDomain: domain, Subrole: subrole})
		certificates[id] = RoleCertificate(t, key)
	}
}
