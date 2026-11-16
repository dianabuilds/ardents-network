package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceModeRejectsIncompleteInvocation(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"source"}, {"source", "--config"}, {"role", "--config", "x"},
		{"contributor", "diagnose"}, {"contributor", "restart"},
		{"contributor", "apply", "--bundle", "b", "--manifest-pin", "p"},
		{"contributor", "remove", "--confirm", "x"}} {
		if err := run(context.Background(), arguments, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
}

func TestNodeOwnedPlansRejectRetiredH3Schemas(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		arguments []string
	}{
		{"source-server", []string{"source", "--config", "retired-plan.json"}},
		{"node", []string{"node", "--config", "retired-plan.json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "retired-plan.json")
			if err := os.WriteFile(path, []byte(`{"schema":"ardents-h3-plan-v1"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			arguments := append([]string(nil), test.arguments...)
			arguments[len(arguments)-1] = path
			if err := run(t.Context(), arguments, new(bytes.Buffer)); err == nil {
				t.Fatalf("retired H3 %s plan schema was accepted", test.name)
			}
		})
	}
}

func TestNodeConfigRetirementPrecedesPlanEffects(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "native-legacy-resource.json")
	raw := `{"schema":"ardents-node-plan-v1","local_role_state_root":"role","authority_public":["00"],"sources":[{},{}],"node_resource_profile":"h3-np1-v1","rendezvous":{}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"node", "--config", path}, new(bytes.Buffer)); !errors.Is(err, errOldNodeConfigRetired) {
		t.Fatalf("native plan with legacy resource profile error = %v", err)
	}
}
