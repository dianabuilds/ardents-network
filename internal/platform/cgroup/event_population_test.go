package cgroup

import "testing"

func TestPopulationIsIndependentOfFreezerState(t *testing.T) {
	for _, tc := range []struct {
		body      string
		populated bool
	}{
		{"populated 1\nfrozen 0\n", true},
		{"populated 0\nfrozen 1\n", false},
	} {
		got, err := parseEvents([]byte(tc.body))
		if err != nil || got != tc.populated {
			t.Fatalf("independent kernel facts: %q got=%v want=%v err=%v", tc.body, got, tc.populated, err)
		}
	}
}
