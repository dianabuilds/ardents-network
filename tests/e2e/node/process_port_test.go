package state_test

import (
	"net"
	"testing"
)

// reservedProcessPort prevents fixture listeners from reusing each other's
// addresses during preparation. The child still binds normally after release;
// this is not an atomic listener transfer or protection from external binders.
type reservedProcessPort struct {
	address  string
	listener net.Listener
}

func reserveProcessPort(t *testing.T) *reservedProcessPort {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := &reservedProcessPort{address: listener.Addr().String(), listener: listener}
	t.Cleanup(func() { port.release(t) })
	return port
}

func (port *reservedProcessPort) release(t *testing.T) {
	t.Helper()
	if port.listener == nil {
		return
	}
	if err := port.listener.Close(); err != nil {
		t.Fatal(err)
	}
	port.listener = nil
}

func TestProcessPortReservationsRetainOtherAddresses(t *testing.T) {
	var ports [4]*reservedProcessPort
	t.Run("owned listeners", func(t *testing.T) {
		for index := range ports {
			ports[index] = reserveProcessPort(t)
			for earlier := range index {
				if ports[earlier].address == ports[index].address {
					t.Fatal("live reservations share an address")
				}
			}
		}
		ports[0].release(t)
		listener, err := net.Listen("tcp", ports[0].address)
		if err != nil {
			t.Fatalf("released address remains occupied: %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		for _, port := range ports[1:] {
			listener, err := net.Listen("tcp", port.address)
			if err == nil {
				_ = listener.Close()
				t.Fatal("another reservation was released with the first")
			}
		}
	})
	for _, port := range ports {
		listener, err := net.Listen("tcp", port.address)
		if err != nil {
			t.Fatalf("cleanup retained a reservation: %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
