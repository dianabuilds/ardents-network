package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceModeRejectsIncompleteInvocation(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"source"}, {"source", "--config"}, {"role", "--config", "x"}} {
		if err := run(context.Background(), arguments, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
}

func TestContributorModeAcceptsOnlyItsClosedLifecycleGrammar(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"diagnose"}, {"restart"}, {"drain"}, {"withdraw"},
		{"remove", "--confirm", strings.Repeat("11", 32)},
		{"apply", "--bundle", "/bundle", "--manifest-pin", strings.Repeat("12", 32)},
	} {
		if _, err := parseContributorRequest(arguments); err != nil {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
	for _, arguments := range [][]string{nil, {"start"}, {"remove"}, {"remove", "--confirm", ""}, {"apply", "--manifest-pin", "x", "--bundle", "/bundle"}} {
		if _, err := parseContributorRequest(arguments); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
}

func TestNodeOwnedPlansRejectRetiredH3Schemas(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		raw  string
		read func(string) error
	}{
		{"source-server", `{"schema":"ardents-h3-source-server-v1"}`, func(path string) error { _, err := openSource(path, nil); return err }},
		{"node", `{"schema":"ardents-h3-node-plan-v1"}`, func(path string) error { _, err := readNodePlan(path); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "retired-plan.json")
			if err := os.WriteFile(path, []byte(test.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := test.read(path); err == nil {
				t.Fatalf("retired H3 %s plan schema was accepted", test.name)
			}
		})
	}
}

func TestNodePlanRejectsLegacyResourceProfileForNativeDuty(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "native-legacy-resource.json")
	raw := `{"schema":"ardents-node-plan-v1","local_role_state_root":"role","authority_public":["00"],"sources":[{},{}],"node_resource_profile":"h3-np1-v1","rendezvous":{}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readNodePlan(path); err == nil || !strings.Contains(err.Error(), "resource profile is unselected") {
		t.Fatalf("native plan with legacy resource profile error = %v", err)
	}
}

func TestNodePlanAcceptsDedicatedHostResourceProfileOnlyForRendezvous(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		profile    string
		duty       string
		wantDetail string
	}{
		{name: "canonical Rendezvous", profile: "ardents-rendezvous-dedicated-host-v1", duty: `"rendezvous":{}`, wantDetail: "invalid fixed hexadecimal value"},
		{name: "legacy Rendezvous", profile: "h4-5-rendezvous-alpha-v1", duty: `"rendezvous":{}`, wantDetail: "invalid fixed hexadecimal value"},
		{name: "canonical Initiator", profile: "ardents-rendezvous-dedicated-host-v1", duty: `"initiator":{}`, wantDetail: "requires only one Rendezvous duty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "functional-alpha-resource.json")
			raw := `{"schema":"ardents-node-plan-v1","local_role_state_root":"role","authority_public":["00"],"sources":[{},{}],"node_resource_profile":"` + test.profile + `",` + test.duty + `}`
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readNodePlan(path)
			if err == nil || !strings.Contains(err.Error(), test.wantDetail) {
				t.Fatalf("%s plan error = %v", test.name, err)
			}
		})
	}
}
