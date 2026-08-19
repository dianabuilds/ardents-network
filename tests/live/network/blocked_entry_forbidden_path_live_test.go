//go:build live

package network_test

import (
	"encoding/json"
	"testing"
)

type forbiddenPathChallenge struct {
	Variant   string `json:"variant"`
	Source    string `json:"source"`
	Component string `json:"component"`
}

type finalForbiddenPathReceipt struct {
	Schema                  string          `json:"schema"`
	Variant                 string          `json:"variant"`
	Source                  string          `json:"source"`
	Component               string          `json:"component"`
	InputSHA256             string          `json:"input_sha256"`
	Calls                   uint16          `json:"calls"`
	ContactStarts           uint16          `json:"contact_starts"`
	Terminal                string          `json:"terminal"`
	DeadlineOffset          uint64          `json:"deadline_offset"`
	CandidateContract       json.RawMessage `json:"candidate_contract"`
	CandidateContractSHA256 string          `json:"candidate_contract_sha256"`
}

type g7ComponentContract struct {
	Schema           string          `json:"schema"`
	Variant          string          `json:"variant"`
	Component        string          `json:"component"`
	Input            json.RawMessage `json:"input"`
	ReachableTargets []string        `json:"reachable_targets"`
	ObservedTargets  []string        `json:"observed_targets"`
	ChildEnvironment []string        `json:"child_environment"`
	StateEntries     []string        `json:"state_entries"`
	EntryError       string          `json:"entry_error"`
}

func finalForbiddenPathChallenge(variant string) (forbiddenPathChallenge, bool) {
	values := map[string][2]string{
		"dns":                     {"host-alias", "adapter-resolver"},
		"environment-proxy":       {"proxy-environment", "adapter-process"},
		"ordinary-entry":          {"ordinary-entry-address", "route-entry"},
		"direct-target":           {"direct-target-address", "endpoint-route"},
		"alternate-address":       {"uncommitted-address", "adapter-config"},
		"alternate-candidate":     {"uncommitted-candidate", "bridge-ledger"},
		"shorter-route":           {"short-route-address", "route-plan"},
		"cached-success":          {"prior-success-cache", "adapter-state"},
		"deadline-exposure-reset": {"reset-request", "bridge-attempt"},
	}
	value, ok := values[variant]
	return forbiddenPathChallenge{Variant: variant, Source: value[0], Component: value[1]}, ok
}

func TestEveryForbiddenPathVariantHasADistinctAmbientChallenge(t *testing.T) {
	variants := []string{"dns", "environment-proxy", "ordinary-entry", "direct-target", "alternate-address",
		"alternate-candidate", "shorter-route", "cached-success", "deadline-exposure-reset"}
	seen := make(map[string]bool, len(variants))
	for _, variant := range variants {
		challenge, ok := finalForbiddenPathChallenge(variant)
		if !ok || challenge.Variant != variant || challenge.Source == "" || challenge.Component == "" ||
			seen[challenge.Source] {
			t.Fatalf("variant %q challenge=%+v ok=%t", variant, challenge, ok)
		}
		seen[challenge.Source] = true
	}
	if _, ok := finalForbiddenPathChallenge("unknown"); ok {
		t.Fatal("unknown forbidden path acquired an ambient challenge")
	}
}
