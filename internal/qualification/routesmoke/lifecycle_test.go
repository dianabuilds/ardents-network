package routesmoke

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureCleanupRequiresOwnershipAndRemovesCredentials(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fixture")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := removeFixture(root); err == nil {
		t.Fatal("unowned fixture cleanup passed")
	}
	if err := os.WriteFile(filepath.Join(root, ".ardents-route-smoke-owned"), []byte(fixtureOwner), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "key.pem"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeFixture(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("owned fixture remains: %v", err)
	}
}

func TestEvidenceBundleBindsManifestVerifierAndCleanup(t *testing.T) {
	root := t.TempDir()
	for name, value := range map[string][]byte{
		"preflight.json": []byte("source"), "manifest.json": []byte("manifest"),
		"evidence.jsonl": []byte("evidence"), "verdict.json": []byte("pass"), "cleanup.json": []byte("cleanup"),
	} {
		if err := os.WriteFile(filepath.Join(root, name), value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	first, err := bundleDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "verdict.json"), []byte("fail"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := bundleDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first[:], second[:]) {
		t.Fatal("outer evidence digest ignored verifier result")
	}
}

func TestVerifierVerdictsAndConcurrentProjectsStayDistinct(t *testing.T) {
	root := t.TempDir()
	for _, verdict := range []string{"pass", "fail", "invalid"} {
		raw := []byte(`{"verdict":"` + verdict + `","reason":"test"}`)
		got, err := acceptVerifier(root, raw)
		if err != nil || got != verdict {
			t.Fatalf("verifier %s = %s, %v", verdict, got, err)
		}
	}
	source := "0123456789012345678901234567890123456789"
	if projectName(source, [32]byte{1}) == projectName(source, [32]byte{2}) {
		t.Fatal("concurrent fixtures share one Compose project")
	}
}

func TestTerminalEvidenceRequiresExactlyOneCompleteRecord(t *testing.T) {
	complete := []byte("{\"kind\":\"complete\",\"terminal\":\"success\"}\n")
	if _, _, err := terminalEvidence(nil); err == nil || verdict(err) != "invalid" {
		t.Fatalf("missing complete record = %v", err)
	}
	duplicate := append(append([]byte(nil), complete...), complete...)
	if _, _, err := terminalEvidence(duplicate); err == nil || verdict(err) != "invalid" {
		t.Fatalf("duplicate complete records = %v", err)
	}
}

func TestRootValidationResolvesSymlinkParents(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked-source")
	if err := os.Symlink(source, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	input := Config{FixtureRoot: filepath.Join(t.TempDir(), "fixture"),
		EvidenceRoot: filepath.Join(link, "evidence"), SourceRoot: source}
	if err := validateRoots(input); err == nil {
		t.Fatal("evidence through source symlink passed root validation")
	}
}

func TestInvalidInputDoesNotOverwritePriorEvidence(t *testing.T) {
	root := t.TempDir()
	summary := filepath.Join(root, "summary.json")
	if err := os.WriteFile(filepath.Join(root, ".ardents-route-smoke-evidence"), []byte(evidenceOwner), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summary, []byte("prior"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Run(t.Context(), Config{FixtureRoot: filepath.Join(t.TempDir(), "fixture"), EvidenceRoot: root,
		ComposeFile: filepath.Join(t.TempDir(), "compose.yaml"), SourceRoot: t.TempDir(), Duration: 10 * time.Minute})
	if result.Verdict != "invalid" {
		t.Fatalf("existing evidence root = %+v", result)
	}
	raw, err := os.ReadFile(summary)
	if err != nil || string(raw) != "prior" {
		t.Fatalf("prior summary changed to %q: %v", raw, err)
	}
}

func TestMissingCurrentEvidenceCannotFinalizePass(t *testing.T) {
	result := finalizeEvidence(Config{EvidenceRoot: filepath.Join(t.TempDir(), "missing")}, Result{Verdict: "pass"})
	if result.Verdict != "invalid" || result.EvidenceDigest != "" {
		t.Fatalf("missing evidence finalized as %+v", result)
	}
}

func TestNilExclusionsKeepCanonicalManifestForm(t *testing.T) {
	if hexIDs(nil) != nil {
		t.Fatal("nil identity exclusions became a non-nil empty slice")
	}
}
