package recoverysmoke

import "testing"

func TestRecoveryTerminalWaitOrderStartsWithMeasuredReceiver(t *testing.T) {
	for _, receiver := range []string{"client-app", "publisher-app"} {
		order, err := recoveryTerminalWaitOrder(receiver)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != len(recoveryServiceNames()) || order[0] != receiver {
			t.Fatalf("receiver=%s order=%v", receiver, order)
		}
		seen := make(map[string]bool, len(order))
		for _, service := range order {
			if seen[service] {
				t.Fatalf("receiver=%s duplicated service %s", receiver, service)
			}
			seen[service] = true
		}
	}
	if _, err := recoveryTerminalWaitOrder("client-endpoint"); err == nil {
		t.Fatal("non-Application terminal marker was accepted")
	}
}
