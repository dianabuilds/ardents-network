package siteexperiment

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedFailureProbeSetupErrorIsOperational(t *testing.T) {
	err := runContractFailure(t.Context(), "forbidden_origin_query_role_view", t.TempDir())
	if !errors.Is(err, errMatrixOperational) || errors.Is(err, errFailureAssertion) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing retained evidence was misclassified: %v", err)
	}
}

func TestEveryGateCFailureProbeProducesBoundedEvidence(t *testing.T) {
	evidence := t.TempDir()
	writeFailureFixtureEvidence(t, evidence)
	for _, name := range fixedFailureCases {
		t.Run(name, func(t *testing.T) {
			if err := runContractFailure(context.Background(), name, evidence); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(filepath.Join(evidence, "failures", name+".json"))
			if err != nil || info.Size() <= 0 || info.Size() > 1024*1024 {
				t.Fatal("failure probe did not retain bounded evidence")
			}
		})
	}
}

func writeFailureFixtureEvidence(t *testing.T, evidence string) {
	t.Helper()
	reference := filepath.Join(evidence, "attempts", "001", "reference")
	values := map[string]any{
		"relay/relay.json": map[string]any{
			"schema_version": "gatec-resolution-role-view/v1", "exact_name_or_target_visible": false,
		},
		"gateway/gateway.json": map[string]any{
			"schema_version": "gatec-resolution-role-view/v1", "plaintext_query_types": []string{"name", "reachability"}, "authority_private_key_present": false,
		},
		"isolation.json": map[string]any{
			"schema_version": "gatec-isolation-evidence/v1", "application_network_mode_none": true, "principal_filesystem_views": true, "published_ports": false,
			"active_dns_escape_rejected": true, "active_socket_escape_rejected": true, "active_listener_absent": true,
		},
		"http-application/http-application.json": map[string]any{
			"schema_version": "gatec-http-application-evidence/v1", "ordinary_listener": false,
		},
	}
	for relative, value := range values {
		path := filepath.Join(reference, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := writeBoundedJSON(path, value); err != nil {
			t.Fatal(err)
		}
	}
}
