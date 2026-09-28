//go:build linux

package forwarding

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type closedForwardingHostReservation = hosting.Reservation

func closedForwardRecipient(config forwardingFixtureConfig, snapshot state.NodeDuty, open route.ClosedOpen, now time.Time) (state.NodeDutyCandidate, error) {
	return recipient(config.source(), snapshot, open, now, literalForwardingFixtureEndpoint)
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}
