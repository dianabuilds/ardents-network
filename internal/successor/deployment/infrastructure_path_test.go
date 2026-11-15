package deployment

import (
	"strings"
	"testing"
)

func TestInfrastructurePathRules(t *testing.T) {
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"/var/lib/ardents", true},
		{"/var/lib/../ardents", false},
		{"/", false},
		{"relative", false},
		{"/var/lib/ardents%node", false},
		{"/var/é", false},
		{"/" + strings.Repeat("x", 4096), false},
		{strings.Repeat("/x", 128), false},
	} {
		if got := ValidInfrastructurePath(tc.path); got != tc.valid {
			t.Errorf("path %q: got %v, want %v", tc.path, got, tc.valid)
		}
	}
	for _, tc := range []struct {
		parent, child string
		contains      bool
	}{
		{"/root/a", "/root/a", true},
		{"/root/a", "/root/a/child", true},
		{"/root/a", "/root/ab", false},
		{"/root/a/child", "/root/a", false},
	} {
		if got := InfrastructurePathContains(tc.parent, tc.child); got != tc.contains {
			t.Errorf("%q contains %q: got %v", tc.parent, tc.child, got)
		}
	}
}

func TestIndependentNodePathsRejectsOverlap(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		valid bool
	}{
		{[]string{"/data/state", "/data/spend", "/credentials/cert", "/credentials/key"}, true},
		{[]string{"/data/state", "/data/state"}, false},
		{[]string{"/data/state", "/data/state/spend"}, false},
		{[]string{"/data/state/spend", "/data/state"}, false},
		{[]string{"/data/state", "relative"}, false},
	} {
		if got := IndependentNodePaths(tc.paths...); got != tc.valid {
			t.Fatalf("paths %v: got %v, want %v", tc.paths, got, tc.valid)
		}
	}
}

func TestPlanOutputKeepsRequestAndOtherNamespacesIndependent(t *testing.T) {
	const output, plan, name = "/prepared", "/prepared/join-plan.json", "join-plan.json"
	for _, tc := range []struct {
		request, candidate string
		paths              []string
		valid              bool
	}{
		{"/request", plan, []string{plan, "/state", "/credentials"}, true},
		{"/request", "/prepared/foreign.json", []string{plan, "/state"}, false},
		{"/prepared/request", plan, []string{plan, "/state"}, false},
		{"/state/request", plan, []string{plan, "/state"}, false},
		{"/request", plan, []string{plan, "/prepared/other"}, false},
		{"/request", plan, []string{plan, "/prepared"}, false},
		{"relative", plan, []string{plan, "/state"}, false},
	} {
		if got := PlanOutputIndependent(output, tc.request, tc.candidate, name, tc.paths); got != tc.valid {
			t.Fatalf("request %q plan %q paths %v: got %v", tc.request, tc.candidate, tc.paths, got)
		}
	}
}
