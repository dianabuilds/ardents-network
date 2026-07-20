package status

import "testing"

func TestLifecycleForHealth(t *testing.T) {
	cases := map[string]string{
		"ready":    "ready",
		"degraded": "degraded",
		"failed":   "failed",
		"other":    "ready",
	}
	for health, want := range cases {
		if got := LifecycleForHealth(health, "ready", "degraded", "failed"); got != want {
			t.Fatalf("health %q -> %q, want %q", health, got, want)
		}
	}
}

func TestRuntimeFailure(t *testing.T) {
	if err := RuntimeFailure("start", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RuntimeFailure("start", true, "broken"); err == nil || err.Error() != "node start failed: broken" {
		t.Fatalf("err = %v", err)
	}
	if err := RuntimeFailure("stop", true, ""); err == nil || err.Error() != "node stop failed" {
		t.Fatalf("err = %v", err)
	}
}

func TestPrimaryReasonHelpers(t *testing.T) {
	if got := PrimaryReasonSummary("boot degraded", false); got != "" {
		t.Fatalf("summary = %q, want empty without primary", got)
	}
	if got := PrimaryReasonCode("boot.join.degraded", false); got != "" {
		t.Fatalf("code = %q, want empty without primary", got)
	}
	if got := PrimaryReasonSummary("boot degraded", true); got != "boot degraded" {
		t.Fatalf("summary = %q, want preserved summary", got)
	}
	if got := PrimaryReasonCode("boot.join.degraded", true); got != "boot.join.degraded" {
		t.Fatalf("code = %q, want preserved code", got)
	}
}

func TestPrimaryReasonOwnershipRules(t *testing.T) {
	if !CanAdoptPrimaryReason("", "boot") {
		t.Fatal("empty current domain must allow adoption")
	}
	if !CanAdoptPrimaryReason("boot", "boot") {
		t.Fatal("same owner domain must allow adoption")
	}
	if CanAdoptPrimaryReason("discovery", "boot") {
		t.Fatal("different owner domain must reject adoption")
	}
	if !IsObservedPrimaryDomain("boot") || !IsObservedPrimaryDomain("transport") {
		t.Fatal("boot and transport must be observed primary domains")
	}
	if IsObservedPrimaryDomain("discovery") {
		t.Fatal("discovery must not be observed primary domain")
	}
	if !ShouldClearPrimaryOnStop("publication") {
		t.Fatal("publication stop reason must be cleared")
	}
	if ShouldClearPrimaryOnStop("policy") {
		t.Fatal("policy stop reason must not be cleared")
	}
}
