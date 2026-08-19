//go:build live

package network_test

import (
	"os"
	"testing"
)

func TestEveryG5VariantSelectsItsExactTerminal(t *testing.T) {
	previous := os.Getenv("ARDENTS_FINAL_CELL")
	t.Cleanup(func() { _ = os.Setenv("ARDENTS_FINAL_CELL", previous) })
	for _, test := range []struct{ variant, terminal string }{
		{"slow-partial-handshake", "bridge-attempt-exhausted"},
		{"malformed-pt-control", "bridge-attempt-exhausted"},
		{"wrong-socks-listener-method", "bridge-attempt-exhausted"},
		{"child-exit", "bridge-attempt-exhausted"}, {"sigterm", "bridge-attempt-exhausted"},
		{"sigkill", "bridge-attempt-exhausted"}, {"accept-then-stall", "bridge-attempt-exhausted"},
		{"malformed-carriage", "bridge-attempt-exhausted"},
		{"evidence-write-exhaustion", "bridge-local-denial"},
	} {
		cell := "hostile/G5-adapter-fault/" + test.variant + "/3"
		if err := os.Setenv("ARDENTS_FINAL_CELL", cell); err != nil {
			t.Fatal(err)
		}
		got, terminal, ok := selectedG5FinalCell(3)
		if !ok || got != cell || terminal != test.terminal ||
			finalWorkerTest(cell) != "TestBlockedEntryNegativeCommandsAcrossNamespaces" {
			t.Fatalf("G5 %s dispatch=%q terminal=%q ok=%t", test.variant, got, terminal, ok)
		}
	}
}

func TestEveryG6AndG7VariantHasAWorker(t *testing.T) {
	for _, cell := range []string{
		"hostile/G6-substitution/target/0", "hostile/G6-substitution/instance-generation/0",
		"hostile/G6-substitution/isolation-context/0", "hostile/G6-substitution/route-generation/0",
		"hostile/G6-substitution/attachment/0", "hostile/G6-substitution/application-canary/0",
		"hostile/G7-forbidden-path/dns/0", "hostile/G7-forbidden-path/environment-proxy/0",
		"hostile/G7-forbidden-path/ordinary-entry/0", "hostile/G7-forbidden-path/direct-target/0",
		"hostile/G7-forbidden-path/alternate-address/0", "hostile/G7-forbidden-path/alternate-candidate/0",
		"hostile/G7-forbidden-path/shorter-route/0", "hostile/G7-forbidden-path/cached-success/0",
		"hostile/G7-forbidden-path/deadline-exposure-reset/0",
	} {
		_, terminal, ok := selectedHostileContractCell(cell)
		if !ok || terminal == "" || finalWorkerTest(cell) != "TestBlockedEntryFinalHostileBindingAndPath" {
			t.Fatalf("hostile contract cell %s is not executable", cell)
		}
	}
}

func TestEveryProductG8VariantHasAWorker(t *testing.T) {
	for _, cell := range []string{"hostile/G8-lifecycle/cancellation/0",
		"hostile/G8-lifecycle/expiry-revocation/0", "hostile/G8-lifecycle/endpoint-restart/0",
		"hostile/G8-lifecycle/bridge-restart/0", "hostile/G8-lifecycle/clock-discontinuity/0",
		"hostile/G8-lifecycle/residual-injection/0"} {
		if finalWorkerTest(cell) == "" {
			t.Fatalf("G8 cell %s is not executable", cell)
		}
	}
}
