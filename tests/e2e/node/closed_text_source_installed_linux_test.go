//go:build linux && text_worker_installed

package state_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The ordinary Endpoint opens its own Source client and retains its actual
// exposure history. Test code supplies only operator inputs and TLS files.
func installedCommandSourcePlan(t *testing.T, directory string, nodePlan map[string]any) string {
	t.Helper()
	copyInput := func(name string, input any) string {
		t.Helper()
		raw, err := os.ReadFile(input.(string))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	sources := make([]map[string]any, 0, 2)
	for index, source := range nodePlan["sources"].([]map[string]any) {
		member := make(map[string]any, len(source))
		for key, value := range source {
			member[key] = value
		}
		member["root_ca"] = copyInput(fmt.Sprintf("source-%d-ca.pem", index), source["root_ca"])
		sources = append(sources, member)
	}
	plan := map[string]any{
		"schema": "ardents-source-plan-v1", "network_id": nodePlan["network_id"],
		"authority_public": nodePlan["authority_public"], "threshold": nodePlan["threshold"],
		"clock_observed_at":      time.Now().UTC().Format(time.RFC3339),
		"clock_observation_file": filepath.Join(directory, "clock"),
		"order_seed":             nodePlan["order_seed"], "materialization_index": 0,
		"refresh_interval_ms": 1000, "local_role_state_root": filepath.Join(directory, "roles"),
		"client_certificate": copyInput("source-client.pem", nodePlan["source_client_certificate"]),
		"client_key":         copyInput("source-client-key.pem", nodePlan["source_client_key"]), "sources": sources,
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "sources.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
