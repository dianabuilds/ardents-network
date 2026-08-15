package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var errTestInfrastructure = errors.New("observer unavailable")

func TestNextAttemptPermitsOnlyInfrastructureInvalidRetry(t *testing.T) {
	root := t.TempDir()
	if attempt, receipt, err := NextAttempt(root, "isolated-rendezvous"); err != nil || receipt != nil || attempt != "attempt-0001" {
		t.Fatalf("first attempt=%q err=%v", attempt, err)
	}
	broken := successfulAdapter()
	broken.arm = func(context.Context) error { return errTestInfrastructure }
	if _, err := RunCell(context.Background(), CellInput{CellID: "isolated-rendezvous", AttemptID: "attempt-0001",
		ManifestDigest: strings.Repeat("a", 64), ReceiptRoot: root}, broken); err != nil {
		t.Fatal(err)
	}
	if attempt, receipt, err := NextAttempt(root, "isolated-rendezvous"); err != nil || receipt != nil || attempt != "attempt-0002" {
		t.Fatalf("retry attempt=%q err=%v", attempt, err)
	}
}

func TestNextAttemptRejectsRetryAfterCandidateTerminal(t *testing.T) {
	for _, candidate := range []string{candidatePass, candidateFail} {
		t.Run(candidate, func(t *testing.T) {
			root := t.TempDir()
			adapter := successfulAdapter()
			adapter.observe = func(context.Context) (CellObservation, error) {
				return CellObservation{Candidate: candidate, TerminalAt: successfulTerminal()}, nil
			}
			adapter.freeze = func(context.Context) (FrozenCell, error) {
				return FrozenCell{Candidate: candidate, Evidence: []byte(`{"complete":true}`)}, nil
			}
			if _, err := RunCell(context.Background(), CellInput{CellID: "terminal", AttemptID: "attempt-0001",
				ManifestDigest: strings.Repeat("b", 64), ReceiptRoot: root}, adapter); err != nil {
				t.Fatal(err)
			}
			attempt, receipt, err := NextAttempt(root, "terminal")
			if err != nil || attempt != "" || receipt == nil || receipt.Candidate != candidate {
				t.Fatalf("terminal candidate lookup = %q %+v %v", attempt, receipt, err)
			}
		})
	}
}

func successfulTerminal() time.Time { return time.Now().Add(time.Nanosecond) }
