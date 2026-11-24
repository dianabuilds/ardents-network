package processfixture

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// OriginalListenerOccupied probes the signed endpoint with the same Carrier.
func OriginalListenerOccupied(t *testing.T, member network.Member) bool {
	t.Helper()
	var closer interface{ Close() error }
	var err error
	if member.CarrierProfile == epoch.CarrierClosedTCP {
		closer, err = net.Listen("tcp", member.Endpoint)
	} else if member.CarrierProfile == epoch.CarrierClosedQUIC {
		closer, err = net.ListenPacket("udp", member.Endpoint)
	} else {
		t.Fatal("unselected native listener probe")
	}
	if err == nil {
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
		return false
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatal("original native endpoint observation unavailable", err)
	}
	return true
}

// RequireFailedStartupCleanup verifies that a refused start joined its real owners.
func RequireFailedStartupCleanup(t *testing.T, participationRoot, spendRoot, hostingRoot string, member network.Member, binding spending.Binding) {
	t.Helper()
	if OriginalListenerOccupied(t, member) {
		t.Fatal("rejected original acquisition retained its actual listener")
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participationRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
		t.Fatal("rejected original startup left a producer", conflict, err, closeErr)
	}
	ledger, err := spending.Open(spendRoot, binding)
	if err != nil {
		t.Fatal("late startup returned before its purpose root joined", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(hostingRoot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("late rejected acquisition retained capacity", snapshot, err, closeErr)
	}
}

// AwaitNativeExpiryDenial waits for the original process to deny a genuinely expired duty.
func AwaitNativeExpiryDenial(t *testing.T, actual network.RuntimeView, retained network.RetainedDuty, failed func() bool) error {
	t.Helper()
	now := time.Now()
	expiry := actual.MatchDuty(retained, now)
	if expiry == nil || now.Before(retained.RecordValidUntil) {
		t.Fatal("genuine original duty remained current after signed expiry")
	}
	// Keep physical return held until denial is known. A scheduler delay alone
	// cannot determine whether producer or post-I/O MatchDuty observes expiry first.
	until := time.Now().Add(2 * time.Second)
	for !failed() {
		if time.Now().After(until) {
			t.Fatal("original expiry denial unavailable while physical return remained held")
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
	return expiry
}
