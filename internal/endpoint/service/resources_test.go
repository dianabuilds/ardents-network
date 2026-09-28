//go:build linux

package service

import "testing"

func TestResourceObserverRetainsOwnedHighWater(t *testing.T) {
	ledger := NewResourceObserver()
	ledger("accepted-ipc", 1)
	ledger("accepted-ipc", 1)
	ledger("timer", 1)
	ledger("control-file", 1)
	ledger("service-connection", 1)
	ledger("accepted-ipc", -1)
	ledger("accepted-ipc", -1)
	ledger("timer", -1)
	ledger("control-file", -1)
	ledger("service-connection", -1)

	if ledger("accepted-ipc", 0) != 2 || ledger("timer", 0) != 1 || ledger("control-file", 0) != 1 ||
		ledger("service-connection", 0) != 1 {
		t.Fatal("resource observer did not retain exact high-water values")
	}
}
