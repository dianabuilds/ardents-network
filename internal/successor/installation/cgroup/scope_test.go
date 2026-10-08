package cgroup

import (
	"strings"
	"testing"
)

func TestInstallationCgroupEventsRequireCompleteKernelObservation(t *testing.T) {
	for _, test := range []struct {
		body      string
		populated bool
		valid     bool
	}{
		{"populated 0\nfrozen 0\n", false, true},
		{"frozen 1\npopulated 1\n", true, true},
		{"", false, false},
		{"populated 0\n", false, false},
		{"populated 0\npopulated 0\n", false, false},
		{"populated 0\nfrozen 0\nother 0\n", false, false},
		{"populated 2\nfrozen 0\n", false, false},
		{"populated 0\nfrozen 2\n", false, false},
		{strings.Repeat(" ", 1025), false, false},
	} {
		populated, err := parseEvents([]byte(test.body))
		if (err == nil) != test.valid || populated != test.populated {
			t.Fatalf("%q: populated=%v error=%v", test.body, populated, err)
		}
	}
}

func TestInstallationWorkerScopeOriginalIdentity(t *testing.T) {
	for _, unit := range []string{
		"ardents-text-reader@0-123-996.service",
		"ardents-text-publisher@4294967295-123-996.service",
	} {
		if !workerMatches(unit, 123, 996) || !scopeUnit(unit) {
			t.Fatalf("canonical original worker refused: %s", unit)
		}
		if workerMatches(unit, 124, 996) || workerMatches(unit, 123, 997) {
			t.Fatalf("replacement process or account adopted: %s", unit)
		}
	}
	for _, unit := range []string{
		"ardents-endpoint.service",
		"ardents-text-reader@01-123-996.service",
		"ardents-text-reader@1-0123-996.service",
		"ardents-text-reader@1-123-0996.service",
		"ardents-text-reader@1-0-996.service",
		"ardents-text-reader@1-123-0.service",
		"ardents-text-reader@4294967296-123-996.service",
		"ardents-text-reader@1-123-996-1.service",
		"ardents-text-reader@1-123-996.scope",
		"ardents-text-writer@1-123-996.service",
		"ardents-qualification-reader@1-123-996.service",
		"../ardents-text-reader@1-123-996.service",
	} {
		if workerMatches(unit, 123, 996) {
			t.Fatalf("foreign scope admitted: %s", unit)
		}
		if unit != "ardents-endpoint.service" && scopeUnit(unit) {
			t.Fatalf("noncanonical scope admitted: %s", unit)
		}
	}
}
