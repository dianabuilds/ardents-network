package hosting

import "testing"

func TestJointTrafficUsesActualCountedDirections(t *testing.T) {
	for _, test := range []struct {
		direction string
		expected  uint64
	}{{"tx", 200}, {"rx", 150}, {"tx+rx", 200}} {
		cost, err := (JointTraffic{Tx: 200, Rx: 150, Total: 200}).cost(Policy{Directions: test.direction})
		if err != nil || cost != test.expected {
			t.Fatalf("%s: %d / %v", test.direction, cost, err)
		}
	}
	for _, invalid := range []JointTraffic{{}, {Tx: 200, Rx: 200, Total: 100}, {Tx: 100, Rx: 50, Total: 200}} {
		if _, err := invalid.cost(Policy{Directions: "tx+rx"}); err == nil {
			t.Fatal("contradictory envelope accepted", invalid)
		}
	}
}
