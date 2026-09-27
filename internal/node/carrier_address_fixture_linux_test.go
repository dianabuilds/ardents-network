//go:build linux

package node

import (
	"net"
	"testing"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// The receiving-Node tests use the same local address fixture on both hosts.
func reserveClosedBootstrapAddress(t *testing.T, carrier routecarrier.CarrierProfile) string {
	t.Helper()
	if carrier == routecarrier.ClosedCarrierTCP {
		return reserveAddress(t)
	}
	socket, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := socket.LocalAddr().String()
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}
