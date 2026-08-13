package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCampaignManifestSealCoversImmutableInputs(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	for _, directory := range []string{"artifacts", "plans", "secrets", "state"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, directory, "input"), []byte(directory), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"manifest.json", ".ardents-node-manifest.sha256"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	observer := nodeObserver{input: Campaign{FixtureRoot: root, EvidenceRoot: evidence, Mode: "short"}, sourceDigest: "source"}
	if err := observer.freezeCampaignManifest([]byte("manifest.json"), []byte("compose")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "compose-resolved.yaml"), []byte("compose"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeCampaignManifest(evidence, root, "short", "source", observer.initialStateDigest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "compose-resolved.yaml"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeCampaignManifest(evidence, root, "short", "source", observer.initialStateDigest); err == nil {
		t.Fatal("changed frozen Compose file passed validation")
	}
	if err := os.WriteFile(filepath.Join(evidence, "compose-resolved.yaml"), []byte("compose"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(evidence, "campaign-manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest nodeCampaignManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.InitialStateSHA256 = "rewritten"
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	seal := []byte(hex.EncodeToString(digest[:]) + "\n")
	if err := os.WriteFile(filepath.Join(evidence, nodeCampaignManifestSeal), seal, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeCampaignManifest(evidence, root, "short", "source", observer.initialStateDigest); err == nil {
		t.Fatal("rewritten initial-state digest passed final manifest validation")
	}
}

func TestCampaignManifestRejectsSymlinkedFixtureInput(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		if errors.Is(err, os.ErrPermission) || runtime.GOOS == "windows" {
			t.Skip("symlinks require an elevated Windows token")
		}
		t.Fatal(err)
	}
	if _, err := collectNodeFixtureFiles(root, 2); err == nil {
		t.Fatal("symlinked fixture input was accepted")
	}
}

func TestCampaignManifestBoundsFixtureInputCount(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := collectNodeFixtureFiles(root, 1); err == nil {
		t.Fatal("unbounded fixture input set was accepted")
	}
}

func TestChurnResourceScheduleCoversBothProfiles(t *testing.T) {
	want := [][2]string{{"node1", "memory"}, {"node2", "cpu"}, {"node1", "memory"},
		{"node2", "cpu"}, {"node1", "memory"}, {"source", "cpu"}}
	for index, expected := range want {
		service, pressure := churnResourceCell(index + 1)
		if service != expected[0] || pressure != expected[1] {
			t.Fatalf("cycle %d = %s/%s, want %s/%s", index+1, service, pressure, expected[0], expected[1])
		}
	}
}

func TestHarnessCleanupFailureIsInvalid(t *testing.T) {
	verdict, err := classifyNodeCleanup("pass", nil, errors.New("docker query failed"))
	if verdict != "invalid" || err == nil {
		t.Fatalf("cleanup outcome=%q err=%v, want invalid", verdict, err)
	}
}

func TestCandidateFailurePreservesHarnessIdentity(t *testing.T) {
	cause := errors.New("docker control failed")
	err := nodeCandidateFailure("candidate timeout", invalidNodeCampaign(cause))
	if !errors.Is(err, errInvalidNodeCampaign) || !errors.Is(err, cause) {
		t.Fatalf("error identity lost: %v", err)
	}
	if err := nodeCandidateFailure("candidate timeout", errors.New("deadline")); err.Error() != "candidate timeout" {
		t.Fatalf("candidate error = %v", err)
	}
	if err := nodeCandidateFailure("candidate timeout", context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation identity lost: %v", err)
	}
}

func TestCampaignVerdictSeparatesCandidateAndEvidenceFailures(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{nil, "pass"},
		{errors.New("candidate contract failed"), "fail"},
		{invalidNodeCampaign(errors.New("docker unavailable")), "invalid"},
		{context.Canceled, "invalid"},
		{context.DeadlineExceeded, "invalid"},
	} {
		if got := nodeCampaignVerdict(test.err); got != test.want {
			t.Fatalf("verdict for %v = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestSampleErrorPolicyKeepsDiagnosticsNonAuthoritative(t *testing.T) {
	diagnostic := errors.New("candidate log stream changed during restart")
	fatal, retained := classifyNodeSampleErrors(nil, nil, diagnostic)
	if fatal != nil || !errors.Is(retained, diagnostic) {
		t.Fatalf("sample errors = fatal %v, diagnostic %v", fatal, retained)
	}
	resource := errors.New("external resource observer failed")
	fatal, retained = classifyNodeSampleErrors(nil, resource, diagnostic)
	if !errors.Is(fatal, resource) || !errors.Is(retained, diagnostic) {
		t.Fatalf("sample errors = fatal %v, diagnostic %v", fatal, retained)
	}
}

func TestRestartReadinessUsesOnlyEventsAfterBarrier(t *testing.T) {
	historical := []byte("{\"kind\":\"lifecycle\",\"state\":\"READY\"}\n" +
		"{\"kind\":\"lifecycle\",\"state\":\"READY\"}\n")
	current := []byte("{\"kind\":\"lifecycle\",\"state\":\"READY\"}\n")
	if countNodeLogEvents(current, "", "READY") > countNodeLogEvents(historical, "", "READY") {
		t.Fatal("fixture no longer reproduces the mismatched-window false negative")
	}
	if !nodeSetReadyAfterRestart([2][]byte{current, current}) {
		t.Fatal("fresh READY events were rejected because historical counts were larger")
	}
}
