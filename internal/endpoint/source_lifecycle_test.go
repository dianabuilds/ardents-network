//go:build linux

package endpoint

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestTextSourceHandleRejectsUseAfterIdleRetirement(t *testing.T) {
	endpoint, owner, _ := startRoleNetwork(t, roleNetworkFixture{carrier: carrier.ClosedCarrierTCP})
	defer func() { _ = endpoint.Close() }()
	handle, err := owner.openPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := closeSourceHandle(handle); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	err = owner.retirePrefixLocked()
	retired := owner.source.CurrentLocked() == nil
	owner.mu.Unlock()
	if err != nil || !retired {
		t.Fatalf("idle Source retirement failed: retired=%v err=%v", retired, err)
	}
	if _, _, _, err := handle.DataJoinRecipient(); err == nil {
		t.Fatal("retired Source handle remained usable")
	}
}
