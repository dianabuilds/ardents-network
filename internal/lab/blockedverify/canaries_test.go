package blockedverify

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repeatedCandidateCanary(value string) string { return strings.Repeat(value, 64) }

func TestScanPublishableClassifiesCandidateCanaryAsFailure(t *testing.T) {
	root := t.TempDir()
	evidencePath := filepath.Join(root, "evidence.json")
	canary := make([]byte, 32)
	for index := range canary {
		canary[index] = byte(index + 1)
	}
	encoded := hex.EncodeToString(canary)
	if err := os.WriteFile(evidencePath, []byte(`{"diagnostic":"`+encoded+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, failures := scanPublishable(root, evidencePath, filepath.Join(root, "verdict.json"),
		[][]byte{canary}, []string{encoded}, map[string]bool{encoded: true})
	if len(invalid) != 0 || len(failures) != 1 {
		t.Fatalf("candidate leakage classification invalid=%v failures=%v", invalid, failures)
	}
	invalid, failures = scanPublishable(root, evidencePath, filepath.Join(root, "verdict.json"),
		[][]byte{canary}, []string{encoded}, nil)
	if len(invalid) != 1 || len(failures) != 0 {
		t.Fatalf("unattributed leakage classification invalid=%v failures=%v", invalid, failures)
	}
}

func TestCandidateCanaryExerciseRequiresAllFourBoundSecrets(t *testing.T) {
	set := canarySet{Variant: "candidate-leak-invite", Invite: repeatedCandidateCanary("1"),
		Address: repeatedCandidateCanary("2"), Path: repeatedCandidateCanary("3"), Certificate: repeatedCandidateCanary("4")}
	corpus := canaryCorpus{Sets: []canarySet{set}}
	observed := map[string]bool{set.Invite: true, set.Address: true, set.Path: true}
	if reasons := verifyCandidateCanaryExercise("candidate-canary-invite", corpus, observed); len(reasons) != 1 {
		t.Fatalf("partial candidate leakage exercise reasons=%v", reasons)
	}
	observed[set.Certificate] = true
	if reasons := verifyCandidateCanaryExercise("candidate-canary-invite", corpus, observed); len(reasons) != 0 {
		t.Fatalf("complete candidate leakage exercise reasons=%v", reasons)
	}
}
