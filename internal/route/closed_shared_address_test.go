package route

import (
	"testing"
)

func closedSharedCarrierEndpoint(t *testing.T, listener ClosedSharedCarrierListener) string {
	t.Helper()
	switch value := listener.(type) {
	case *closedSharedTCPListener:
		return value.listener.Addr().String()
	case *closedSharedQUICListener:
		return value.listener.Addr().String()
	default:
		t.Fatal("unknown closed shared listener")
		return ""
	}
}
