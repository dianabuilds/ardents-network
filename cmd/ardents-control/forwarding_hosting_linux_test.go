package main

import "testing"

func TestForwardingHostingArgumentsKeepOneExplicitPeriod(t *testing.T) {
	args := []string{"--binding", "/opt/ardents-forwarding/binding.json", "--provider", "declared provider", "--start", "2026-10-27T00:00:00Z", "--end", "2026-11-27T00:00:00Z", "--unit", "GiB", "--quantity", "100", "--directions", "tx+rx", "--interfaces", "eth0,lo", "--initial-used-bytes", "1000", "--low-watermark-bytes", "1048576"}
	path, policy, err := forwardingHostingArguments(args)
	if err != nil || path != "/opt/ardents-forwarding/binding.json" || policy.InitialUsedBytes != 1000 || len(policy.Interfaces) != 2 {
		t.Fatal("explicit period refused", err)
	}
	for _, change := range []struct {
		position int
		value    string
	}{{1, "relative"}, {5, "2026-10-27T00:00:00.5Z"}, {7, "2026-10-26T00:00:00Z"}, {9, "unknown"}, {11, "0"}, {13, "both"}, {15, "eth0,eth0"}, {19, "0"}} {
		candidate := append([]string(nil), args...)
		candidate[change.position] = change.value
		if _, _, err := forwardingHostingArguments(candidate); err == nil {
			t.Fatal("invalid period accepted", change)
		}
	}
	for _, candidate := range [][]string{args[:18], append(append([]string(nil), args...), "--quantity", "1"), append(append([]string(nil), args...), "--root", "/another/allowance")} {
		if _, _, err := forwardingHostingArguments(candidate); err == nil {
			t.Fatal("missing/duplicate/foreign allowance accepted")
		}
	}
}
