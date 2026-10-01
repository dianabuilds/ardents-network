//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func completedJournalFixture(t *testing.T, outcome error) []byte {
	t.Helper()
	var output bytes.Buffer
	journal := newEvidenceJournal(&output)
	for _, value := range []any{
		map[string]any{"Kind": "candidate", "Participants": 1, "PlanSHA256": strings.Repeat("ab", 32)},
		map[string]any{"Participant": 0, "Event": map[string]string{"Kind": "resource-sample"}},
		map[string]any{"Kind": "result", "Participant": 0},
	} {
		if err := journal.emit(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := journal.finish(outcome); err != nil {
		t.Fatal(err)
	}
	if err := journal.emit("late"); err == nil {
		t.Fatal("record accepted after cleanup terminal")
	}
	return output.Bytes()
}

func readEvidenceFixture(t *testing.T, body []byte) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "attempt.jsonl")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return readCompletedEvidence(path, func([]byte) error { return nil })
}

func TestQualificationEvidenceRequiresCompleteJoinedAttempt(t *testing.T) {
	body := completedJournalFixture(t, nil)
	if err := readEvidenceFixture(t, body); err != nil {
		t.Fatal(err)
	}
	lastLine := bytes.LastIndex(body[:len(body)-1], []byte("\n")) + 1
	for name, changed := range map[string][]byte{
		"missing-terminal":    body[:lastLine],
		"partial-terminal":    body[:len(body)-3],
		"missing-newline":     body[:len(body)-1],
		"altered-observation": bytes.Replace(body, []byte("resource-sample"), []byte("resource-tamper"), 1),
		"post-terminal":       append(append([]byte(nil), body...), []byte("{}\n")...),
		"cleanup-failure":     completedJournalFixture(t, errors.New("worker cleanup timed out")),
	} {
		t.Run(name, func(t *testing.T) {
			if err := readEvidenceFixture(t, changed); err == nil {
				t.Fatal("incomplete or changed attempt accepted")
			}
		})
	}
}

func TestQualificationEvidenceRequiresEveryParticipantCleanupOnce(t *testing.T) {
	for _, indices := range [][]int{{0}, {0, 0}, {0, 2}} {
		var output bytes.Buffer
		journal := newEvidenceJournal(&output)
		if err := journal.emit(map[string]any{"Kind": "candidate", "Participants": 2, "PlanSHA256": strings.Repeat("ab", 32)}); err != nil {
			t.Fatal(err)
		}
		for _, index := range indices {
			if err := journal.emit(map[string]any{"Kind": "result", "Participant": index}); err != nil {
				t.Fatal(err)
			}
		}
		if err := journal.finish(nil); err != nil {
			t.Fatal(err)
		}
		if err := readEvidenceFixture(t, output.Bytes()); err == nil {
			t.Fatal("invalid participant result set accepted")
		}
	}
}

type qualificationShortWriter struct{}

func (qualificationShortWriter) Write(body []byte) (int, error) { return len(body) - 1, nil }

func TestQualificationEvidenceCannotSealAfterOutputFailure(t *testing.T) {
	journal := newEvidenceJournal(qualificationShortWriter{})
	if err := journal.emit("record"); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write not retained")
	}
	if err := journal.finish(nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("terminal erased output failure")
	}
}

func TestQualificationEarlyArtifactFailureRetainsTerminal(t *testing.T) {
	var output bytes.Buffer
	err := run(t.Context(), []string{"/tmp/not-the-installed-qualification-plan.json"}, &output)
	if err == nil || !strings.Contains(err.Error(), "fixed installed plan") {
		t.Fatalf("early artifact refusal = %v", err)
	}
	var terminal evidenceTerminal
	if decodeErr := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &terminal); decodeErr != nil || terminal.Kind != "terminal" || terminal.Records != 0 ||
		terminal.SHA256 != hex.EncodeToString(sha256.New().Sum(nil)) || !strings.Contains(terminal.Failure, "fixed installed plan") {
		t.Fatalf("early refusal lacks its terminal record: %+v / %v\n%s", terminal, decodeErr, output.Bytes())
	}
}
