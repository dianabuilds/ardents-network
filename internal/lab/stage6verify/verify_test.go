package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierVerifyCompleteCampaignAndRejectsMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEvidenceCampaign(t, root, "source-commit", "clean")
	verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
	if verdict.Status != "pass" || len(verdict.Diagnostics) != 0 {
		t.Fatalf("verdict=%+v", verdict)
	}
	trace := filepath.Join(root, "evidence", "cells", "00", "observations", "trace.jsonl")
	file, err := os.OpenFile(trace, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	mutated := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict-mutated"))
	if mutated.Status != "invalid" {
		t.Fatalf("mutated verdict=%+v", mutated)
	}
}

func TestStage6VerifierDistinguishesSemanticFailureAndStructuralContamination(t *testing.T) {
	root := t.TempDir()
	writeEvidenceCampaign(t, root, "source-commit", "clean")
	if err := rewriteSemanticMutation(root); err != nil {
		t.Fatal(err)
	}
	failed := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"), filepath.Join(root, "evidence"),
		filepath.Join(root, "private"), filepath.Join(root, "verdict-fail"))
	if failed.Status != "fail" || len(failed.Diagnostics) != 1 || failed.Diagnostics[0] != "A0:predicate-false" {
		t.Fatalf("semantic mutation verdict=%+v", failed)
	}

	contaminated := t.TempDir()
	writeEvidenceCampaign(t, contaminated, "source-commit", "clean")
	if err := os.WriteFile(filepath.Join(contaminated, "evidence", "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(contaminated, "manifest"),
		filepath.Join(contaminated, "evidence"), filepath.Join(contaminated, "private"), filepath.Join(contaminated, "verdict"))
	if invalid.Status != "invalid" {
		t.Fatalf("contaminated verdict=%+v", invalid)
	}
	nested := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(contaminated, "manifest"),
		filepath.Join(contaminated, "evidence"), filepath.Join(contaminated, "private"),
		filepath.Join(contaminated, "evidence", "verdict"))
	if nested.Status != "invalid" || len(nested.Diagnostics) != 1 || nested.Diagnostics[0] != "root-separation" {
		t.Fatalf("nested-root verdict=%+v", nested)
	}
}

type mutationTrace struct {
	Schema      string   `json:"schema"`
	Cell        string   `json:"cell"`
	Ordinal     uint32   `json:"ordinal"`
	Operation   string   `json:"operation"`
	Input       []byte   `json:"input"`
	Output      []byte   `json:"output"`
	Auxiliary   []byte   `json:"auxiliary"`
	Values      []int64  `json:"values"`
	Fields      []string `json:"fields"`
	StartOffset int64    `json:"start_offset_millis"`
	EndOffset   int64    `json:"end_offset_millis"`
}

type mutationArtifact struct {
	Path   string `json:"path"`
	Schema string `json:"schema"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type mutationCell struct {
	ID             string                        `json:"id"`
	Ordinal        uint32                        `json:"ordinal"`
	EpisodeOrdinal uint32                        `json:"episode_ordinal"`
	Streams        []mutationObservationArtifact `json:"streams"`
	TerminalClass  string                        `json:"terminal_class"`
	Terminal       mutationArtifact              `json:"terminal"`
	Cleanup        mutationArtifact              `json:"cleanup"`
}

type mutationObservationArtifact struct {
	Path             string `json:"path"`
	Schema           string `json:"schema"`
	Role             string `json:"role"`
	EpisodeOrdinal   uint32 `json:"episode_ordinal"`
	StreamOrdinal    uint32 `json:"stream_ordinal"`
	ObservationStart int64  `json:"observation_start_millis"`
	ObservationEnd   int64  `json:"observation_end_millis"`
	Size             int64  `json:"size"`
	SHA256           string `json:"sha256"`
}

type mutationIndex struct {
	Schema         string         `json:"schema"`
	CampaignSHA256 string         `json:"campaign_sha256"`
	Cells          []mutationCell `json:"cells"`
}

func rewriteSemanticMutation(root string) error {
	tracePath := filepath.Join(root, "evidence", "cells", "00", "observations", "trace.jsonl")
	raw, err := os.ReadFile(tracePath)
	if err != nil {
		return err
	}
	var trace mutationTrace
	if err = json.Unmarshal(raw, &trace); err != nil {
		return err
	}
	trace.Output[0]++
	raw, err = json.Marshal(trace)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = os.WriteFile(tracePath, raw, 0o600); err != nil {
		return err
	}
	indexPath := filepath.Join(root, "evidence", "index.json")
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var index mutationIndex
	if err = json.Unmarshal(indexRaw, &index); err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	index.Cells[0].Streams[0].Size = int64(len(raw))
	index.Cells[0].Streams[0].SHA256 = hex.EncodeToString(digest[:])
	indexRaw, err = json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, indexRaw, 0o600)
}
