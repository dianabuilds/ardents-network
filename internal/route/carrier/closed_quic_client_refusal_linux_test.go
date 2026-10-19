//go:build linux

package carrier

import (
	"strings"
	"testing"
	"time"
)

func TestDeferredQUICRoleClientRefusesBeforeDial(t *testing.T) {
	connection, err := OpenClosedRoleCarrier(t.Context(), ClosedRoleCarrierRequest{CarrierProfile: ClosedCarrierQUIC,
		Endpoint: "127.0.0.1:9", ExpectedServer: identifier(1), Deadline: time.Now().Add(time.Second)})
	if connection != nil || err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("want explicit profile refusal before dial, got %v / %v", connection, err)
	}
}
