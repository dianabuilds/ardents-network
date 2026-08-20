package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierPublishesExactBoundVerdictOnce(t *testing.T) {
	root := t.TempDir()
	writeEvidenceCampaign(t, root, "source-commit", "clean")
	verdictRoot := filepath.Join(root, "verdict")
	verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), verdictRoot)
	if verdict.Status != "pass" {
		t.Fatalf("verdict=%+v", verdict)
	}
	assertVerdictDigest(t, filepath.Join(root, "manifest", "campaign.json"), verdict.CampaignSHA256)
	assertVerdictDigest(t, filepath.Join(root, "evidence", "index.json"), verdict.EvidenceSHA256)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	assertVerdictDigest(t, executable, verdict.VerifierSHA256)
	raw, err := os.ReadFile(filepath.Join(verdictRoot, "verdict.json"))
	if err != nil {
		t.Fatal(err)
	}
	var published stage6verify.Verdict
	if err = json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(published)
	if err != nil || !reflect.DeepEqual(published, verdict) || string(raw) != string(canonical) {
		t.Fatalf("published=%+v returned=%+v canonical=%t err=%v", published, verdict, string(raw) == string(canonical), err)
	}

	occupied := filepath.Join(root, "occupied-verdict")
	if err = os.Mkdir(occupied, 0o700); err != nil {
		t.Fatal(err)
	}
	repeated := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), occupied)
	if repeated.Status != "invalid" || len(repeated.Diagnostics) != 1 || repeated.Diagnostics[0] != "verdict-root-invalid" {
		t.Fatalf("repeated verdict=%+v", repeated)
	}
}

func assertVerdictDigest(t *testing.T, path, expected string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if got := hex.EncodeToString(digest[:]); got != expected {
		t.Fatalf("%s digest=%s want=%s", path, got, expected)
	}
}
