package carrier

import (
	"net"
	"testing"
)

func closedRoleCarrierTestEndpoint(t *testing.T, profile CarrierProfile) string {
	t.Helper()
	switch profile {
	case ClosedCarrierTCP:
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		endpoint := listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return endpoint

	default:
		t.Fatal("unknown role carrier profile")
		return ""
	}
}
