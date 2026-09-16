package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualificationEvidenceReportsAllObservableDefects(t *testing.T) {
	body := []byte(
		"{\"Kind\":\"candidate\",\"Participants\":2,\"PlanSHA256\":\"bad\"}\n" +
			"{\"Kind\":\"result\",\"Participant\":7}\n" +
			"{\"Kind\":\"unexpected\",\"Participant\":0}\n" +
			"{\"Kind\":\"terminal\",\"Records\":99,\"SHA256\":\"bad\",\"Failure\":\"cleanup failed\"}\n",
	)
	path := filepath.Join(t.TempDir(), "attempt.jsonl")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	err := readCompletedEvidence(path, func([]byte) error { return nil })
	if err == nil {
		t.Fatal("multiply invalid evidence accepted")
	}
	for _, want := range []string{
		"candidate plan identity",
		"duplicated or invalid result participant",
		"unexpected kind",
		"runner ended with failure",
		"checksum or record count differs",
		"cleanup results incomplete",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregate error %q lacks %q", err, want)
		}
	}
}
