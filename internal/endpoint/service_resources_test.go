package endpoint

import "testing"

func TestResourceObserverRetainsOwnedHighWater(t *testing.T) {
	ledger := newResourceObserver()
	releaseIPC1 := acquireResource(ledger, "accepted-ipc")
	releaseIPC2 := acquireResource(ledger, "accepted-ipc")
	releaseTimer := acquireResource(ledger, "timer")
	releaseFile := acquireResource(ledger, "control-file")
	releaseConnection := acquireResource(ledger, "service-connection")
	releaseIPC1()
	releaseIPC1()
	releaseIPC2()
	releaseTimer()
	releaseFile()
	releaseConnection()

	if ledger("accepted-ipc", 0) != 2 || ledger("timer", 0) != 1 || ledger("control-file", 0) != 1 ||
		ledger("service-connection", 0) != 1 {
		t.Fatal("resource observer did not retain exact high-water values")
	}
}
