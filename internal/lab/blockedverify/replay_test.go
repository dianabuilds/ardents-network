package blockedverify

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplayRegistryCommitsOneRecoverableTransaction(t *testing.T) {
	parent := t.TempDir()
	registry := filepath.Join(parent, "registry")
	bundle := filepath.Join(parent, "bundle")
	output := filepath.Join(bundle, "verdict.json")
	transaction, _, reason := beginRun(registry, "run-1", "nonce-1", "manifest-1", "verdict-1", bundle, output)
	if reason != "" {
		t.Fatalf("first consume: %s", reason)
	}
	if err := transaction.commit(); err != nil {
		t.Fatal(err)
	}
	if transaction, _, reason := beginRun(registry, "run-1", "nonce-2", "manifest-2", "verdict-2", bundle, output); reason == "" {
		transaction.abandon()
		t.Fatal("duplicate run identity was accepted")
	}
	if err := os.WriteFile(filepath.Join(registry, ".consumed-next"), []byte("crash"), 0o600); err != nil {
		t.Fatal(err)
	}
	transaction, _, reason = beginRun(registry, "run-2", "nonce-2", "manifest-2", "verdict-2", bundle, output)
	if reason != "" {
		t.Fatalf("stale uncommitted transaction was not recovered: %s", reason)
	}
	if err := transaction.commit(); err != nil {
		t.Fatal(err)
	}
	var state replayRegistry
	if _, err := decodeStrict(filepath.Join(registry, "consumed.json"), &state); err != nil || len(state.Entries) != 2 {
		t.Fatalf("registry state entries=%d err=%v", len(state.Entries), err)
	}
}

func TestPendingReplayTransactionRecoversPublishedVerdict(t *testing.T) {
	parent := t.TempDir()
	registry, bundle := filepath.Join(parent, "registry"), filepath.Join(parent, "bundle")
	output := filepath.Join(bundle, "verdict.json")
	result := Result{Schema: "ardents-h3-blocked-entry-verdict-v1", RunID: "run", Verdict: "pass",
		ManifestSHA256: "manifest"}
	verdictHash, err := canonicalDecisionHash(result)
	if err != nil {
		t.Fatal(err)
	}
	transaction, published, reason := beginRun(registry, "run", "nonce", "manifest", verdictHash, bundle, output)
	if reason != "" || published {
		t.Fatalf("reservation published=%v reason=%s", published, reason)
	}
	if _, err := finish(output, result, nil); err != nil {
		t.Fatal(err)
	}
	transaction.abandon()
	transaction, published, reason = beginRun(registry, "run", "nonce", "manifest", "new-verdict", bundle, output)
	if reason != "" || !published {
		t.Fatalf("recovery published=%v reason=%s", published, reason)
	}
	if recovered, err := recoverPublished(output, "manifest", "run", verdictHash); err != nil || recovered.Verdict != "pass" {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
	if err := transaction.commit(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingReplayRejectsChangedUnpublishedDecision(t *testing.T) {
	parent := t.TempDir()
	registry, bundle := filepath.Join(parent, "registry"), filepath.Join(parent, "bundle")
	output := filepath.Join(bundle, "verdict.json")
	transaction, _, reason := beginRun(registry, "run", "nonce", "manifest", "decision-1", bundle, output)
	if reason != "" {
		t.Fatal(reason)
	}
	transaction.abandon()
	if transaction, _, reason := beginRun(registry, "run", "nonce", "manifest", "decision-2", bundle, output); reason == "" {
		transaction.abandon()
		t.Fatal("changed pending decision was accepted")
	}
}

func TestPendingReplayRemovesStaleVerdictTemporaryBeforePublication(t *testing.T) {
	parent := t.TempDir()
	registry, bundle := filepath.Join(parent, "registry"), filepath.Join(parent, "bundle")
	output := filepath.Join(bundle, "verdict.json")
	transaction, _, reason := beginRun(registry, "run", "nonce", "manifest", "decision", bundle, output)
	if reason != "" {
		t.Fatal(reason)
	}
	if err := os.MkdirAll(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output+".tmp", []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	transaction.abandon()
	transaction, published, reason := beginRun(registry, "run", "nonce", "manifest", "decision", bundle, output)
	if reason != "" || published {
		t.Fatalf("recovery published=%v reason=%s", published, reason)
	}
	defer transaction.abandon()
	if _, err := os.Lstat(output + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("stale temporary remains: %v", err)
	}
}

func TestPendingReplayRemovesTemporaryAfterPublishedVerdict(t *testing.T) {
	parent := t.TempDir()
	registry, bundle := filepath.Join(parent, "registry"), filepath.Join(parent, "bundle")
	output := filepath.Join(bundle, "verdict.json")
	result := Result{Schema: "ardents-h3-blocked-entry-verdict-v1", RunID: "run", Verdict: "invalid",
		ManifestSHA256: "manifest", Reasons: []string{"z-reason", "a-reason"}}
	decision, _ := canonicalDecisionHash(result)
	transaction, _, reason := beginRun(registry, "run", "nonce", "manifest", decision, bundle, output)
	if reason != "" {
		t.Fatal(reason)
	}
	if _, err := finish(output, result, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output+".tmp", []byte("durable published bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	transaction.abandon()
	transaction, published, reason := beginRun(registry, "run", "nonce", "manifest", decision, bundle, output)
	if reason != "" || !published {
		t.Fatalf("recovery published=%v reason=%s", published, reason)
	}
	defer transaction.abandon()
	if _, err := os.Lstat(output + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("post-publication temporary remains: %v", err)
	}
	if recovered, err := recoverPublished(output, "manifest", "run", decision); err != nil || recovered.Verdict != "invalid" {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
}

func TestDecisionHashExcludesOnlyPublicationTime(t *testing.T) {
	first := Result{Schema: "ardents-h3-blocked-entry-verdict-v1", Verdict: "pass", VerifiedUnixNano: 1}
	second := first
	second.VerifiedUnixNano = 2
	firstHash, _ := canonicalDecisionHash(first)
	secondHash, _ := canonicalDecisionHash(second)
	if firstHash != secondHash {
		t.Fatal("publication time changed the stable decision hash")
	}
	second.Verdict = "fail"
	secondHash, _ = canonicalDecisionHash(second)
	if firstHash == secondHash {
		t.Fatal("decision change retained the original stable hash")
	}
	first = Result{Schema: "ardents-h3-blocked-entry-verdict-v1", Verdict: "invalid",
		Reasons: []string{"z-reason", "a-reason"}}
	second = first
	second.Reasons = []string{"a-reason", "z-reason"}
	firstHash, _ = canonicalDecisionHash(first)
	secondHash, _ = canonicalDecisionHash(second)
	if firstHash != secondHash {
		t.Fatal("reason iteration order changed the stable decision hash")
	}
}

func TestReplayRegistryEnforcesOwnerOnlyTree(t *testing.T) {
	parent := t.TempDir()
	registry := filepath.Join(parent, "registry")
	if err := os.Mkdir(registry, 0o777); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(registry, 0o777)
	transaction, _, reason := beginRun(registry, "run", "nonce", "manifest", "decision",
		filepath.Join(parent, "bundle"), filepath.Join(parent, "bundle", "verdict.json"))
	if reason != "" {
		t.Fatal(reason)
	}
	if err := transaction.commit(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		err := filepath.Walk(registry, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr == nil && info.Mode().Perm()&0o077 != 0 {
				t.Fatalf("registry path remains permissive: %s %o", path, info.Mode().Perm())
			}
			return walkErr
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
