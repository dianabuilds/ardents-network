//go:build ignore

package main

import (
	"errors"
	"testing"
)

func TestClassifyRefreshRequiresExactPersonaSignature(t *testing.T) {
	t.Parallel()
	honest := personaConfig{Name: "honest_user", ExpectedKind: "accept",
		ExpectedOutcomes: [4]string{"valid", "valid", "not-attempted", "not-attempted"}}
	probe := personaConfig{Name: "probe_consumer", ExpectedKind: "reject",
		ExpectedOutcomes: [4]string{"valid", "invalid-state", "not-attempted", "not-attempted"}}
	honestRaw := testSourceOutput(honest.ExpectedOutcomes)
	event := classifyRefresh(honest, honestRaw, 0, nil)
	if event.Kind != "accept" || event.Generation != "generation-a" || event.ActualOutcomes == nil {
		t.Fatalf("honest classification = %#v", event)
	}
	event = classifyRefresh(probe, testSourceOutput(probe.ExpectedOutcomes), 0, nil)
	if event.Kind != "reject" {
		t.Fatalf("probe classification = %#v", event)
	}
	event = classifyRefresh(probe, honestRaw, 0, nil)
	if event.Kind != "harness_abort" || event.Diagnostic != "unexpected_source_signature" {
		t.Fatalf("mismatched classification = %#v", event)
	}
}

func TestClassifyRefreshSeparatesLocalRoleFailures(t *testing.T) {
	t.Parallel()
	tests := []struct{ message, kind, diagnostic string }{
		{"local role record limit exceeded", "harness_abort", "record_limit"},
		{"local role producer limit exceeded", "harness_abort", "producer_limit"},
		{"local role duty conflicts with retained state", "harness_abort", "conflict"},
		{"local role state exceeds its bound", "harness_abort", "legacy_local_role_validation"},
		{"direct-source exposure set is full", "harness_abort", "installation_source_exhausted"},
		{"finite sources are temporarily unavailable: retry is in durable backoff", "infra_error", "durable_backoff"},
		{"durable source cycle reached its recorded deadline", "infra_error", "cycle_deadline"},
		{"No such container", "infra_error", "container_missing"},
		{"surprising failure", "harness_abort", "unrecognized"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.diagnostic, func(t *testing.T) {
			t.Parallel()
			event := classifyRefresh(personaConfig{}, []byte(test.message), 1, errors.New("exit status 1"))
			if event.Kind != test.kind || event.Diagnostic != test.diagnostic {
				t.Fatalf("classification = %#v", event)
			}
		})
	}
}
