package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAndCheckOneCanonicalCapabilityProjection(t *testing.T) {
	root := catalogueFixture(t)
	require.NoError(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}))
	require.NoError(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}))

	path := filepath.Join(root, "docs", "engineering", "capability-evidence-register.md")
	require.NoError(t, os.WriteFile(path, []byte("drift\n"), 0o600))
	require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "generated projection differs")
}

func TestGeneratePreflightsAllActiveDocumentsBeforeReplacingProjection(t *testing.T) {
	root := catalogueFixture(t)
	path := filepath.Join(root, "docs", "engineering", "capabilities.json")
	var catalogue map[string]any
	require.NoError(t, json.Unmarshal(requireRead(t, path), &catalogue))
	firstCapability(catalogue)["active_documents"] = []any{
		"docs/engineering/capability-evidence-register.md",
		"README.md",
	}
	writeCatalogue(t, root, catalogue)
	register := filepath.Join(root, "docs", "engineering", "capability-evidence-register.md")
	require.NoError(t, os.WriteFile(register, []byte("sentinel\n"), 0o600))

	require.ErrorContains(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}), "stale or missing capability block")
	require.Equal(t, "sentinel\n", string(requireRead(t, register)))
}

func TestGeneratedOutputSetRollsBackAnEarlierReplacement(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a.md")
	second := filepath.Join(root, "b.md")
	require.NoError(t, os.WriteFile(first, []byte("old-a\n"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("old-b\n"), 0o600))
	calls := 0
	injected := func(target string, raw []byte) error {
		calls++
		if calls == 2 {
			return fmt.Errorf("injected replacement failure")
		}
		return atomicWrite(target, raw)
	}

	require.ErrorContains(t, writeOutputSet(map[string][]byte{
		first: []byte("new-a\n"), second: []byte("new-b\n"),
	}, injected), "injected replacement failure")
	require.Equal(t, "old-a\n", string(requireRead(t, first)))
	require.Equal(t, "old-b\n", string(requireRead(t, second)))
}

func TestCatalogueFailsClosedForStructuralAndPromotionErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"empty capabilities", func(c map[string]any) { c["capabilities"] = []any{} }, "catalogue has no capabilities"},
		{"wrong schema", func(c map[string]any) { c["schema_version"] = float64(2) }, "unsupported capability schema_version"},
		{"omitted domain", func(c map[string]any) { c["domains"] = c["domains"].([]any)[1:] }, "required domain application is missing"},
		{"missing initial capability", func(c map[string]any) { c["capabilities"] = c["capabilities"].([]any)[1:] }, "required initial capability"},
		{"moved initial capability", func(c map[string]any) { firstCapability(c)["domain"] = "node" }, "moved from domain application to node"},
		{"missing domain owner", func(c map[string]any) { firstCapability(c)["domain_owner"] = "" }, "domain_owner are required"},
		{"missing implementation owner", func(c map[string]any) { firstCapability(c)["implementation_owners"] = []any{} }, "implementation owner is required"},
		{"missing evidence owner", func(c map[string]any) { firstCapability(c)["evidence_owner"] = "" }, "evidence_owner is required"},
		{"missing interface", func(c map[string]any) { firstCapability(c)["supported_interfaces"] = []any{} }, "supported interface is required"},
		{"unknown status", func(c map[string]any) { firstCapability(c)["status"].(map[string]any)["reachable"] = "operator" }, "unknown reachable status"},
		{"missing research", func(c map[string]any) { firstCapability(c)["research_packets"] = []any{"docs/missing.md"} }, "referenced path does not exist"},
		{"missing ADR", func(c map[string]any) { firstCapability(c)["adrs"] = []any{"docs/adr/missing.md"} }, "referenced path does not exist"},
		{"unknown CI job", func(c map[string]any) { c["evidence_gates"].([]any)[0].(map[string]any)["ci_job"] = "pull_request" }, "unknown CI job"},
		{"unknown environment", func(c map[string]any) {
			c["evidence_gates"].([]any)[0].(map[string]any)["required_environment"] = "invented-environment"
		}, "unknown environment"},
		{"active document outside allowlist", func(c map[string]any) { firstCapability(c)["active_documents"] = []any{"contracts/interface.go"} }, "outside the Markdown allowlist"},
		{"reserved marker injection", func(c map[string]any) {
			firstCapability(c)["user_outcome"] = "<!-- capability-status:begin injected -->"
		}, "user_outcome are required"},
		{"supersession cycle", func(c map[string]any) {
			values := c["capabilities"].([]any)
			first, second := values[0].(map[string]any), values[1].(map[string]any)
			first["supersedes"] = []any{second["id"]}
			second["supersedes"] = []any{first["id"]}
		}, "supersession cycle"},
		{"qualified without snapshot", func(c map[string]any) { firstCapability(c)["status"].(map[string]any)["qualified"] = "yes" }, "qualified capability requires a snapshot"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := catalogueFixture(t)
			path := filepath.Join(root, "docs", "engineering", "capabilities.json")
			var catalogue map[string]any
			require.NoError(t, json.Unmarshal(requireRead(t, path), &catalogue))
			test.change(catalogue)
			raw, err := json.Marshal(catalogue)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, raw, 0o600))
			require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), test.want)
		})
	}
}

func TestStrictLoaderRejectsDuplicateUnknownAndTrailingJSON(t *testing.T) {
	root := catalogueFixture(t)
	path := filepath.Join(root, "docs", "engineering", "capabilities.json")
	raw := string(requireRead(t, path))
	for _, test := range []struct {
		name, raw, want string
	}{
		{"duplicate", strings.Replace(raw, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), "duplicate JSON key"},
		{"unknown", strings.Replace(raw, `"schema_version":1`, `"schema_version":1,"surprise":true`, 1), "unknown field"},
		{"trailing", raw + `{}`, "trailing JSON"},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, []byte(test.raw), 0o600))
			require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), test.want)
		})
	}
	t.Run("invalid UTF-8", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte{0xff}, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "not valid UTF-8")
	})
}

func TestTaggedGateUsesCanonicalCompleteScenarioMetadata(t *testing.T) {
	root := catalogueFixture(t)
	writeFixture(t, root, "tests/fake_test.go", `package fake
import (
	"testing"
	"ardents/tests/testkit"
)
func TestFake(t *testing.T) {
	_ = testkit.BeginScenario(t, testkit.Spec{ScenarioID: "FAKE-001"})
	_ = struct{ ScenarioID string }{ScenarioID: "FAKE-001"}
	// ScenarioID: "FAKE-001"
}
`)
	path := filepath.Join(root, "docs", "engineering", "capabilities.json")
	var catalogue map[string]any
	require.NoError(t, json.Unmarshal(requireRead(t, path), &catalogue))
	catalogue["evidence_gates"] = append(catalogue["evidence_gates"].([]any), map[string]any{
		"id": "fake", "owner": "QA", "kind": "tagged_scenario", "ci_job": "static",
		"scenario_id": "FAKE-001", "required_environment": "ubuntu-latest", "release_gate": false,
	})
	writeCatalogue(t, root, catalogue)
	require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "incomplete scenario metadata")
}

func TestQualificationRequiresOneCompleteCommitBoundEvidenceSet(t *testing.T) {
	root := catalogueFixture(t)
	catalogue := qualifyFirstCapability(t, root)
	writeCatalogue(t, root, catalogue)
	require.NoError(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}))
	require.NoError(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}))

	tests := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"mixed commit", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["source_commit"] = strings.Repeat("b", 40)
		}, "snapshot commit does not match"},
		{"missing gate", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["results"] = c["qualification_snapshots"].([]any)[0].(map[string]any)["results"].([]any)[:1]
		}, "missing gate release"},
		{"hash mismatch", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["results"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
		}, "artifact hash mismatch"},
		{"wrong environment", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["results"].([]any)[0].(map[string]any)["environment"] = "invented"
		}, "wrong environment"},
		{"wrong snapshot environment", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["environment"] = "ubuntu-latest"
		}, "snapshot environment does not match"},
		{"absent artifact", func(c map[string]any) {
			c["qualification_snapshots"].([]any)[0].(map[string]any)["results"].([]any)[0].(map[string]any)["artifact"] = "docs/engineering/evidence/missing.txt"
		}, "referenced path does not exist"},
		{"no required release gate", func(c map[string]any) {
			firstCapability(c)["required_evidence_gates"] = []any{"static"}
			c["qualification_snapshots"].([]any)[0].(map[string]any)["results"] = c["qualification_snapshots"].([]any)[0].(map[string]any)["results"].([]any)[:1]
		}, "omit a release gate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := catalogueFixture(t)
			catalogue := qualifyFirstCapability(t, root)
			test.change(catalogue)
			writeCatalogue(t, root, catalogue)
			require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), test.want)
		})
	}
}

func TestActiveClaimsAndReadinessFailClosed(t *testing.T) {
	t.Run("unknown marker", func(t *testing.T) {
		root := catalogueFixture(t)
		require.NoError(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}))
		readme := filepath.Join(root, "README.md")
		raw := append(requireRead(t, readme), []byte("\n<!-- capability-status:begin unknown.capability -->\n<!-- capability-status:end unknown.capability -->\n")...)
		require.NoError(t, os.WriteFile(readme, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "unknown capability marker")
	})
	t.Run("readiness overclaim guard removed", func(t *testing.T) {
		root := catalogueFixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Product\n"), 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "readiness guard")
	})
	t.Run("readiness overclaim", func(t *testing.T) {
		root := catalogueFixture(t)
		readme := filepath.Join(root, "README.md")
		raw := append(requireRead(t, readme), []byte("\nThe product is production-ready.\n")...)
		require.NoError(t, os.WriteFile(readme, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "rejects production-readiness claims")
	})
	t.Run("accepted release overclaim", func(t *testing.T) {
		root := catalogueFixture(t)
		changelog := filepath.Join(root, "CHANGELOG.md")
		raw := append(requireRead(t, changelog), []byte("\nThis is an accepted production release.\n")...)
		require.NoError(t, os.WriteFile(changelog, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "rejects production-readiness claims")
	})
	t.Run("malformed marker", func(t *testing.T) {
		root := catalogueFixture(t)
		require.NoError(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}))
		readme := filepath.Join(root, "README.md")
		raw := append(requireRead(t, readme), []byte("\n<!-- capability-status:begin invalid_ID -->\n")...)
		require.NoError(t, os.WriteFile(readme, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "malformed capability marker")
	})
	t.Run("stale block", func(t *testing.T) {
		root, _ := activateFirstCapabilityInREADME(t)
		readme := filepath.Join(root, "README.md")
		raw := bytes.Replace(requireRead(t, readme), []byte("A bounded user outcome."), []byte("Stale outcome."), 1)
		require.NoError(t, os.WriteFile(readme, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "stale or missing capability block")
	})
	t.Run("duplicate block", func(t *testing.T) {
		root, item := activateFirstCapabilityInREADME(t)
		readme := filepath.Join(root, "README.md")
		raw := append(requireRead(t, readme), []byte("\n"+renderStatusBlock(item))...)
		require.NoError(t, os.WriteFile(readme, raw, 0o600))
		require.ErrorContains(t, run([]string{"-root", root, "-check"}, &bytes.Buffer{}), "repeats capability marker")
	})
	t.Run("misnested block", func(t *testing.T) {
		root, item := activateFirstCapabilityInREADME(t)
		readme := filepath.Join(root, "README.md")
		raw := fmt.Sprintf("> Status: stabilization candidate, not a production release.\n<!-- capability-status:begin %s -->\n<!-- capability-status:begin %s -->\n<!-- capability-status:end %s -->\n<!-- capability-status:end %s -->\n", item.ID, item.ID, item.ID, item.ID)
		require.NoError(t, os.WriteFile(readme, []byte(raw), 0o600))
		c, err := loadCatalogue(filepath.Join(root, "docs", "engineering", "capabilities.json"))
		require.NoError(t, err)
		require.ErrorContains(t, validateDocumentMarkers(root, declaredDocuments(c), true), "misnested capability markers")
	})
}

func activateFirstCapabilityInREADME(t *testing.T) (string, capability) {
	t.Helper()
	root := catalogueFixture(t)
	path := filepath.Join(root, "docs", "engineering", "capabilities.json")
	var rawCatalogue map[string]any
	require.NoError(t, json.Unmarshal(requireRead(t, path), &rawCatalogue))
	first := firstCapability(rawCatalogue)
	first["active_documents"] = []any{"docs/engineering/capability-evidence-register.md", "README.md"}
	id := first["id"].(string)
	writeCatalogue(t, root, rawCatalogue)
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "README.md"),
		[]byte(fmt.Sprintf("> Status: stabilization candidate, not a production release.\n<!-- capability-status:begin %s -->\n<!-- capability-status:end %s -->\n", id, id)),
		0o600,
	))
	require.NoError(t, run([]string{"-root", root, "-generate"}, &bytes.Buffer{}))
	c, err := loadCatalogue(path)
	require.NoError(t, err)
	return root, c.Capabilities[0]
}

func catalogueFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "contracts/interface.go", "package contracts\n")
	writeFixture(t, root, "implementation/owner.go", "package implementation\n")
	writeFixture(t, root, "docs/adr/0001-test.md", "# ADR\n")
	writeFixture(t, root, "README.md", "> Status: stabilization candidate, not a production release.\n")
	writeFixture(t, root, "CHANGELOG.md", "- no accepted production release or compatibility promise yet;\n")
	writeFixture(t, root, "docs/engineering/capability-evidence-register.md", "# Generated placeholder\n")
	writeFixture(t, root, ".github/workflows/ci.yml", "jobs:\n  static:\n    runs-on: ubuntu-latest\n  release:\n    runs-on: ubuntu-latest\n")
	domains := []any{}
	capabilities := []any{}
	for _, domain := range requiredDomains {
		domains = append(domains, map[string]any{"id": domain, "owner": domain + " owner", "required": true})
	}
	ids := make([]string, 0, len(requiredCapabilities))
	for id := range requiredCapabilities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		domain := requiredCapabilities[id]
		capabilities = append(capabilities, map[string]any{
			"id": id, "user_outcome": "A bounded user outcome.", "domain": domain,
			"domain_owner":          domain + " owner",
			"supported_interfaces":  []any{map[string]any{"id": "interface-v1", "kind": "operator", "contracts": []any{"contracts/interface.go"}}},
			"implementation_owners": []any{"implementation"},
			"operability_surfaces":  []any{"status and recovery"},
			"evidence_owner":        "QA", "required_evidence_gates": []any{"static"},
			"status":         map[string]any{"implemented": "yes", "reachable": "yes", "operable": "yes", "qualified": "no", "at_commit": strings.Repeat("a", 40), "qualification_snapshot": ""},
			"research_class": "R1", "adrs": []any{"docs/adr/0001-test.md"}, "research_packets": []any{}, "backlog": []any{},
			"constraints": []any{"bounded"}, "unsupported_behavior": []any{"production qualification"},
			"active_documents": []any{"docs/engineering/capability-evidence-register.md"},
		})
	}
	catalogue := map[string]any{
		"schema_version": 1, "reported_source_commit": strings.Repeat("a", 40),
		"domains":                 domains,
		"evidence_gates":          []any{map[string]any{"id": "static", "owner": "QA", "kind": "ci_job", "ci_job": "static", "required_environment": "ubuntu-latest", "release_gate": false}},
		"qualification_snapshots": []any{}, "capabilities": capabilities,
	}
	raw, err := json.Marshal(catalogue)
	require.NoError(t, err)
	writeFixture(t, root, "docs/engineering/capabilities.json", string(raw))
	return root
}

func qualifyFirstCapability(t *testing.T, root string) map[string]any {
	t.Helper()
	path := filepath.Join(root, "docs", "engineering", "capabilities.json")
	var catalogue map[string]any
	require.NoError(t, json.Unmarshal(requireRead(t, path), &catalogue))
	artifact := "docs/engineering/evidence/result.txt"
	writeFixture(t, root, artifact, "passed\n")
	sum := sha256.Sum256(requireRead(t, filepath.Join(root, filepath.FromSlash(artifact))))
	catalogue["evidence_gates"] = append(catalogue["evidence_gates"].([]any), map[string]any{
		"id": "release", "owner": "Release", "kind": "release", "ci_job": "release",
		"required_environment": "canonical-linux-release", "release_gate": true,
	})
	snapshotID := "release.snapshot"
	catalogue["qualification_snapshots"] = []any{map[string]any{
		"id": snapshotID, "source_commit": strings.Repeat("a", 40), "environment": "canonical-linux-release", "clean_run": true,
		"results": []any{
			map[string]any{"gate": "static", "outcome": "passed", "environment": "ubuntu-latest", "artifact": artifact, "sha256": fmt.Sprintf("%x", sum)},
			map[string]any{"gate": "release", "outcome": "passed", "environment": "canonical-linux-release", "artifact": artifact, "sha256": fmt.Sprintf("%x", sum)},
		},
	}}
	first := firstCapability(catalogue)
	first["required_evidence_gates"] = []any{"static", "release"}
	first["status"].(map[string]any)["qualified"] = "yes"
	first["status"].(map[string]any)["qualification_snapshot"] = snapshotID
	return catalogue
}

func writeCatalogue(t *testing.T, root string, catalogue map[string]any) {
	t.Helper()
	raw, err := json.Marshal(catalogue)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "engineering", "capabilities.json"), raw, 0o600))
}

func firstCapability(c map[string]any) map[string]any {
	return c["capabilities"].([]any)[0].(map[string]any)
}

func writeFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}

func requireRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return raw
}
