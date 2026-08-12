package node

import (
	"context"
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
	if err := observer.freezeCampaignManifest([]byte("fixture"), []byte("compose")); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeCampaignManifest(evidence, root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "campaign-manifest.json"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNodeCampaignManifest(evidence, root); err == nil {
		t.Fatal("corrupted campaign manifest passed its seal")
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
