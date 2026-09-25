package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

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

func TestContributorOldStartsRefuseBeforeEffects(t *testing.T) {
	hostRoot := t.TempDir()
	installation := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "installation.json")
	if err := os.MkdirAll(filepath.Dir(installation), 0o700); err != nil {
		t.Fatal(err)
	}
	const installationBefore = `{"schema":"ardents-contributor-installation-v1","deployment_id":"owned"}`
	if err := os.WriteFile(installation, []byte(installationBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	supervisor := &contributorSupervisorTrace{}
	environmentCalls := 0
	previousEnvironment := loadContributorHostEnvironment
	loadContributorHostEnvironment = func() (contributorHostEnvironment, error) {
		environmentCalls++
		return contributorHostEnvironment{root: hostRoot, supervisor: supervisor}, nil
	}
	t.Cleanup(func() { loadContributorHostEnvironment = previousEnvironment })

	canonicalBundle, canonicalPin := writeContributorCommandBundle(t, "ardents-rendezvous-dedicated-host-v1", strings.Repeat("31", 32))
	historicalBundle, historicalPin := writeContributorCommandBundle(t, "h4-5-rendezvous-alpha-v1", strings.Repeat("32", 32))
	foreignBundle, foreignPin := writeContributorCommandBundle(t, "ardents-rendezvous-dedicated-host-v1", strings.Repeat("99", 32))
	for _, bundle := range []struct{ path, pin string }{
		{canonicalBundle, canonicalPin},
		{historicalBundle, historicalPin},
		{foreignBundle, foreignPin},
	} {
		raw, err := os.ReadFile(filepath.Join(bundle.path, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != bundle.pin {
			t.Fatal("retired command fixture manifest does not match its pin")
		}
	}
	for _, test := range []struct {
		name      string
		arguments []string
	}{
		{name: "canonical apply", arguments: []string{"contributor", "apply", "--bundle", canonicalBundle, "--manifest-pin", canonicalPin}},
		{name: "historical apply", arguments: []string{"contributor", "apply", "--bundle", historicalBundle, "--manifest-pin", historicalPin}},
		{name: "foreign deployment apply", arguments: []string{"contributor", "apply", "--bundle", foreignBundle, "--manifest-pin", foreignPin}},
		{name: "restart", arguments: []string{"contributor", "restart"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := run(t.Context(), test.arguments, &output)
			if err == nil || err.Error() != "old Contributor start is retired" {
				t.Fatalf("error = %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("output = %q", output.Bytes())
			}
			content, readErr := os.ReadFile(installation)
			if readErr != nil || string(content) != installationBefore {
				t.Fatalf("installation changed: content = %q, error = %v", content, readErr)
			}
		})
	}
	if environmentCalls != 0 {
		t.Fatalf("Contributor host environment opened %d times", environmentCalls)
	}
	if len(supervisor.actions) != 0 {
		t.Fatalf("supervisor actions = %v", supervisor.actions)
	}
}

type contributorSupervisorTrace struct {
	actions []contributor.SupervisorAction
}

func (trace *contributorSupervisorTrace) Do(_ context.Context, action contributor.SupervisorAction) (contributor.SupervisorState, error) {
	trace.actions = append(trace.actions, action)
	return contributor.SupervisorState{}, nil
}

func writeContributorCommandBundle(t *testing.T, profile, deployment string) (string, string) {
	t.Helper()
	bundle := t.TempDir()
	files := map[string][]byte{
		"ardents-node":            []byte("retired-rendezvous-program"),
		"rendezvous-cert.pem":     []byte("rendezvous certificate\n"),
		"rendezvous-key.pem":      []byte("rendezvous key\n"),
		"rendezvous-identity.pem": []byte("rendezvous identity\n"),
		"source-client-cert.pem":  []byte("source client certificate\n"),
		"source-client-key.pem":   []byte("source client key\n"),
		"source-a-root.pem":       []byte("source A root\n"),
		"source-b-root.pem":       []byte("source B root\n"),
		"clock.observation":       []byte("authenticated clock observation\n"),
	}
	plan := map[string]any{
		"schema": "ardents-node-plan-v1", "state_root": "/var/lib/private/ardents-contributor/network",
		"local_role_state_root": "/var/lib/private/ardents-contributor/role", "network_id": strings.Repeat("11", 32),
		"authority_public": []string{strings.Repeat("12", 32)}, "threshold": 1,
		"server_certificate": "/var/lib/private/ardents-contributor/config/current/rendezvous-cert.pem",
		"server_key":         "/var/lib/private/ardents-contributor/config/current/rendezvous-key.pem", "materialization_index": 0,
		"clock_observation_file": "/var/lib/private/ardents-contributor/config/current/clock.observation", "order_seed": strings.Repeat("13", 32),
		"source_client_certificate": "/var/lib/private/ardents-contributor/config/current/source-client-cert.pem",
		"source_client_key":         "/var/lib/private/ardents-contributor/config/current/source-client-key.pem",
		"sources": []map[string]any{
			{"address": "192.0.2.10:48010", "server_name": "source-a.test", "identity": strings.Repeat("14", 32), "family": "source-a", "endpoint_handle": "source-a", "root_ca": "/var/lib/private/ardents-contributor/config/current/source-a-root.pem", "leaf_key_digest": strings.Repeat("15", 32)},
			{"address": "192.0.2.11:48011", "server_name": "source-b.test", "identity": strings.Repeat("16", 32), "family": "source-b", "endpoint_handle": "source-b", "root_ca": "/var/lib/private/ardents-contributor/config/current/source-b-root.pem", "leaf_key_digest": strings.Repeat("17", 32)},
		},
		"node_id": strings.Repeat("18", 32), "identity_key": "/var/lib/private/ardents-contributor/config/current/rendezvous-identity.pem",
		"node_resource_profile": profile, "diagnostic_directory": "/var/lib/private/ardents-contributor/diagnostics",
		"rendezvous": map[string]any{"handshake_limit": 4, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 64 << 20, "admission_timeout_ms": 5000, "drain_timeout_ms": 5000},
	}
	files["node.json"], _ = json.Marshal(plan)
	digests := make(map[string]string, len(files))
	for name, raw := range files {
		mode := os.FileMode(0o600)
		if name == "ardents-node" {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(bundle, name), raw, mode); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		digests[name] = hex.EncodeToString(digest[:])
	}
	manifest, _ := json.Marshal(map[string]any{
		"schema": "ardents-contributor-bundle-v1", "profile": profile,
		"deployment_id": deployment, "generation": 1, "files": digests,
	})
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(manifest)
	return bundle, hex.EncodeToString(pinned[:])
}
