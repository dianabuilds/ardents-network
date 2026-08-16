package blocked_entry_lab_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type harnessResult struct {
	ManifestPath    string `json:"manifest_path"`
	EvidencePath    string `json:"evidence_path"`
	ClosurePath     string `json:"closure_path"`
	SecretRoot      string `json:"secret_root"`
	CanaryPath      string `json:"canary_path"`
	PublishableRoot string `json:"publishable_root"`
	RegistryRoot    string `json:"-"`
}

type verifierResult struct {
	Verdict string   `json:"verdict"`
	Scope   string   `json:"scope"`
	Reasons []string `json:"reasons"`
}

func TestMain(tests *testing.M) {
	if os.Getenv("ARDENTS_BLOCKED_CELL_HELPER") == "1" {
		os.Exit(runCellHelper())
	}
	os.Exit(tests.Run())
}

func TestBlockedEntryVerifierRecomputesPassFailAndInvalid(t *testing.T) {
	workspace := repositoryRoot(t)
	harness := buildCommand(t, workspace, "blocked-entry-lab", "./cmd/blocked-entry-lab")
	verifier := buildCommand(t, workspace, "blocked-entry-verify-lab", "./cmd/blocked-entry-verify-lab")
	supply := t.TempDir()
	client, server := filepath.Join(supply, "client.bin"), filepath.Join(supply, "server.bin")
	if err := os.WriteFile(client, []byte("pinned-client\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server, []byte("pinned-server\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{"pass": "pass", "candidate-fail": "fail", "harness-invalid": "invalid",
		"candidate-canary": "fail", "pipeline-canary": "invalid", "candidate-residual": "fail",
		"candidate-forbidden": "fail", "cell-inventory-missing": "invalid",
		"collector-loss": "invalid", "blocker-loss": "invalid",
		"forbidden-owner-mismatch": "invalid",
		"inventory-missing":        "invalid", "candidate-fail-harness-invalid": "fail"}
	for _, field := range []string{"invite", "address", "path", "certificate"} {
		cases["candidate-canary-"+field] = "fail"
		cases["pipeline-canary-"+field] = "invalid"
	}
	for mode, want := range cases {
		t.Run(mode, func(t *testing.T) {
			bundle := runHarness(t, harness, verifier, workspace, client, server, mode)
			outputPath := filepath.Join(filepath.Dir(bundle.SecretRoot), "verdict.json")
			got := runVerifier(t, verifier, bundle, outputPath)
			if got.Verdict != want {
				t.Fatalf("verdict=%s want=%s", got.Verdict, want)
			}
			if got.Scope != "development-fixture" {
				t.Fatalf("scope=%s", got.Scope)
			}
			if mode == "pass" {
				assertVerifierRefusesOverwrite(t, verifier, bundle, outputPath)
				assertVerifierRejectsAlternateRegistry(t, verifier, bundle, outputPath)
				assertVerifierRejectsConsumedRun(t, verifier, bundle, outputPath)
			}
		})
	}
}

func TestBlockedEntryVerifierRejectsReplayTamperAndMissingEvidence(t *testing.T) {
	workspace := repositoryRoot(t)
	harness := buildCommand(t, workspace, "blocked-entry-lab", "./cmd/blocked-entry-lab")
	verifier := buildCommand(t, workspace, "blocked-entry-verify-lab", "./cmd/blocked-entry-verify-lab")
	supply := t.TempDir()
	client, server := filepath.Join(supply, "client.bin"), filepath.Join(supply, "server.bin")
	_ = os.WriteFile(client, []byte("client\n"), 0o600)
	_ = os.WriteFile(server, []byte("server\n"), 0o600)
	for _, mutation := range []string{"replay", "bundle-copy-replay", "event-omission", "clock-tamper", "unknown-field",
		"observer-missing", "tampered-candidate-fail", "closure-tamper", "secret-tamper",
		"missing-evidence", "missing-closure", "attribution-tamper", "supply-tamper",
		"pipeline-note-removal"} {
		t.Run(mutation, func(t *testing.T) {
			mode := "pass"
			if mutation == "pipeline-note-removal" {
				mode = "pipeline-canary"
			}
			bundle := runHarness(t, harness, verifier, workspace, client, server, mode)
			switch mutation {
			case "replay":
				raw, _ := os.ReadFile(bundle.EvidencePath)
				raw = bytes.Replace(raw, []byte(`"run_id": "run-pass"`), []byte(`"run_id": "replayed"`), 1)
				_ = os.WriteFile(bundle.EvidencePath, raw, 0o600)
			case "bundle-copy-replay":
				copyRoot := filepath.Join(t.TempDir(), "copied-bundle")
				if err := os.CopyFS(copyRoot, os.DirFS(filepath.Dir(bundle.SecretRoot))); err != nil {
					t.Fatal(err)
				}
				bundle.ManifestPath = filepath.Join(copyRoot, "publishable", "manifest.json")
				bundle.EvidencePath = filepath.Join(copyRoot, "publishable", "evidence.json")
				bundle.ClosurePath = filepath.Join(copyRoot, "publishable", "closure.json")
				bundle.SecretRoot = filepath.Join(copyRoot, "secret")
				bundle.CanaryPath = filepath.Join(copyRoot, "secret", "canaries.json")
				bundle.PublishableRoot = filepath.Join(copyRoot, "publishable")
			case "event-omission":
				mutateEvidence(t, bundle.EvidencePath, func(value map[string]any) {
					events := value["events"].([]any)
					value["events"] = events[1:]
				})
			case "clock-tamper":
				mutateEvidence(t, bundle.EvidencePath, func(value map[string]any) {
					event := value["events"].([]any)[0].(map[string]any)
					event["cleanup_offset_millis"] = float64(16_002)
				})
			case "unknown-field":
				mutateEvidence(t, bundle.EvidencePath, func(value map[string]any) {
					value["candidate_verdict"] = "pass"
				})
			case "observer-missing":
				mutateEvidence(t, bundle.EvidencePath, func(value map[string]any) {
					observers := value["observers"].([]any)
					value["observers"] = observers[1:]
				})
			case "tampered-candidate-fail":
				mutateEvidence(t, bundle.EvidencePath, func(value map[string]any) {
					event := value["events"].([]any)[0].(map[string]any)
					event["gate_passed"], event["fault_owner"] = false, "candidate"
					event["observed_terminal"] = "unexpected-success"
				})
			case "closure-tamper":
				raw, _ := os.ReadFile(bundle.ClosurePath)
				raw = bytes.Replace(raw, []byte(`"evidence_sha256": "`), []byte(`"evidence_sha256": "00`), 1)
				_ = os.WriteFile(bundle.ClosurePath, raw, 0o600)
			case "secret-tamper":
				_ = os.WriteFile(filepath.Join(bundle.SecretRoot, "candidate", "client.stderr"), []byte("changed\n"), 0o600)
			case "missing-evidence":
				_ = os.Remove(bundle.EvidencePath)
			case "missing-closure":
				_ = os.Remove(bundle.ClosurePath)
			case "attribution-tamper":
				entries, err := os.ReadDir(filepath.Join(bundle.SecretRoot, "attribution"))
				if err != nil || len(entries) == 0 {
					t.Fatalf("attribution inventory: %v", err)
				}
				path := filepath.Join(bundle.SecretRoot, "attribution", entries[0].Name())
				_ = os.WriteFile(path, []byte("{}\n"), 0o600)
			case "supply-tamper":
				entries, err := os.ReadDir(filepath.Join(bundle.SecretRoot, "supply"))
				if err != nil || len(entries) == 0 {
					t.Fatalf("supply inventory: %v", err)
				}
				path := filepath.Join(bundle.SecretRoot, "supply", entries[0].Name())
				_ = os.Chmod(path, 0o600)
				_ = os.WriteFile(path, []byte("changed-supply\n"), 0o600)
			case "pipeline-note-removal":
				_ = os.Remove(filepath.Join(bundle.PublishableRoot, "pipeline-note.bin"))
			}
			got := runVerifier(t, verifier, bundle, filepath.Join(filepath.Dir(bundle.SecretRoot), "verdict.json"))
			if got.Verdict != "invalid" {
				t.Fatalf("mutation %s verdict=%s", mutation, got.Verdict)
			}
			if mutation == "pipeline-note-removal" && !strings.Contains(strings.Join(got.Reasons, " "), "supplemental") {
				t.Fatalf("pipeline removal reasons=%v", got.Reasons)
			}
		})
	}
}

func TestBlockedEntryVerifierRejectsAlternateBundleRoots(t *testing.T) {
	workspace := repositoryRoot(t)
	harness := buildCommand(t, workspace, "blocked-entry-lab", "./cmd/blocked-entry-lab")
	verifier := buildCommand(t, workspace, "blocked-entry-verify-lab", "./cmd/blocked-entry-verify-lab")
	supply := t.TempDir()
	client, server := filepath.Join(supply, "client.bin"), filepath.Join(supply, "server.bin")
	_ = os.WriteFile(client, []byte("client\n"), 0o600)
	_ = os.WriteFile(server, []byte("server\n"), 0o600)
	bundle := runHarness(t, harness, verifier, workspace, client, server, "pass")
	bundleRoot := filepath.Dir(bundle.SecretRoot)
	alternatePublishable, alternateSecret := filepath.Join(bundleRoot, "altpub"), filepath.Join(bundleRoot, "altsecret")
	if err := os.CopyFS(alternatePublishable, os.DirFS(bundle.PublishableRoot)); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(alternateSecret, os.DirFS(bundle.SecretRoot)); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(verifier, "-manifest", filepath.Join(alternatePublishable, "manifest.json"),
		"-evidence", filepath.Join(alternatePublishable, "evidence.json"),
		"-closure", filepath.Join(alternatePublishable, "closure.json"), "-secret-root", alternateSecret,
		"-registry-root", bundle.RegistryRoot, "-canaries", filepath.Join(alternateSecret, "canaries.json"),
		"-publishable-root", alternatePublishable, "-output", filepath.Join(bundleRoot, "verdict.json"))
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("alternate roots were accepted: %s", output)
	}
	if _, err := os.Lstat(filepath.Join(bundleRoot, "verdict.json")); !os.IsNotExist(err) {
		t.Fatalf("alternate-root rejection published a verdict: %v", err)
	}
}

func mutateEvidence(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	raw, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatalf("write mutated evidence: %v", err)
	}
}

func runHarness(t *testing.T, harness, verifier, workspace, client, server, mode string) harnessResult {
	t.Helper()
	root := filepath.Join(t.TempDir(), "bundle")
	registry := filepath.Join(filepath.Dir(root), "registry")
	command := exec.Command(harness, "-workspace-root", workspace, "-evidence-root", root,
		"-run-id", "run-"+mode, "-mode", mode, "-registry-root", registry,
		"-verifier", verifier, "-client", client, "-server", server)
	command.Args = append(command.Args, "-runner", os.Args[0])
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run harness: %v\n%s", err, output)
	}
	var result harnessResult
	if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
		t.Fatalf("decode harness result: %v\n%s", err, output)
	}
	result.RegistryRoot = registry
	if _, err := os.Stat(root + ".partial"); !os.IsNotExist(err) {
		t.Fatalf("harness construction residue remained: %v", err)
	}
	return result
}

func runVerifier(t *testing.T, verifier string, bundle harnessResult, outputPath string) verifierResult {
	t.Helper()
	command := exec.Command(verifier, "-manifest", bundle.ManifestPath, "-evidence", bundle.EvidencePath,
		"-closure", bundle.ClosurePath, "-secret-root", bundle.SecretRoot, "-registry-root", bundle.RegistryRoot,
		"-canaries", bundle.CanaryPath,
		"-publishable-root", bundle.PublishableRoot, "-output", outputPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run verifier: %v\n%s", err, output)
	}
	var result verifierResult
	if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
		t.Fatalf("decode verifier result: %v\n%s", err, output)
	}
	if raw, err := os.ReadFile(outputPath); err != nil || !strings.Contains(string(raw), `"verdict"`) {
		t.Fatalf("canonical verifier output is absent: %v", err)
	}
	return result
}

func assertVerifierRefusesOverwrite(t *testing.T, verifier string, bundle harnessResult, outputPath string) {
	t.Helper()
	before, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(verifier, "-manifest", bundle.ManifestPath, "-evidence", bundle.EvidencePath,
		"-closure", bundle.ClosurePath, "-secret-root", bundle.SecretRoot, "-registry-root", bundle.RegistryRoot,
		"-canaries", bundle.CanaryPath,
		"-publishable-root", bundle.PublishableRoot, "-output", outputPath)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("verifier replaced an existing verdict: %s", output)
	}
	after, err := os.ReadFile(outputPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("existing verdict changed: %v", err)
	}
}

func assertVerifierRejectsConsumedRun(t *testing.T, verifier string, bundle harnessResult, outputPath string) {
	t.Helper()
	if err := os.Remove(outputPath); err != nil {
		t.Fatal(err)
	}
	got := runVerifier(t, verifier, bundle, outputPath)
	if got.Verdict != "invalid" {
		t.Fatalf("consumed run verdict=%s", got.Verdict)
	}
}

func assertVerifierRejectsAlternateRegistry(t *testing.T, verifier string, bundle harnessResult, outputPath string) {
	t.Helper()
	if err := os.Remove(outputPath); err != nil {
		t.Fatal(err)
	}
	alternate := bundle
	alternate.RegistryRoot = filepath.Join(t.TempDir(), "alternate-registry")
	got := runVerifier(t, verifier, alternate, outputPath)
	if got.Verdict != "invalid" {
		t.Fatalf("alternate replay registry verdict=%s", got.Verdict)
	}
}

func buildCommand(t *testing.T, workspace, name, path string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	command := exec.Command("go", "build", "-trimpath", "-o", binary, path)
	command.Dir = workspace
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return binary
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
}
