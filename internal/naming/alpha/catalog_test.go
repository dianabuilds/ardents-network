package alpha_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestCorpusResolvesOnlyItsCurrentSignedAlphaBinding(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	statement, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: 7,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}},
		NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, statement)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := corpus.Resolve(link, at)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Link() != link || binding.Target() != [32]byte{9} || binding.Serial() != 7 {
		t.Fatalf("resolved alpha binding = %+v", binding)
	}
}

func TestCorpusRefusesWithdrawnAndConflictingAlphaBindings(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	withdrawn, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: 8, Withdrawn: true,
		NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, withdrawn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := corpus.Resolve(link, at); !alpha.HasFailure(err, alpha.FailureWithdrawn) {
		t.Fatalf("withdrawn alpha corpus error = %v, want withdrawn failure", err)
	}

	conflict, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: 9,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}, {Link: link, Target: [32]byte{10}}},
		NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, private)
	if err == nil || conflict != nil {
		t.Fatalf("conflicting alpha corpus issue result = (%x, %v), want failure", conflict, err)
	}
}

func TestCorpusRejectsAStatementFromAnotherAuthority(t *testing.T) {
	t.Parallel()
	trusted, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, foreignPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	statement, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}},
		NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, foreignPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alpha.OpenCorpus(trusted, statement); err == nil {
		t.Fatal("foreign alpha authority statement opened")
	}
}

func TestCorpusDoesNotRemainUsableAfterItsSignedExpiry(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: 7,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}},
		NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Minute)}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := corpus.Resolve(link, at.Add(time.Minute)); !alpha.HasFailure(err, alpha.FailureExpired) {
		t.Fatalf("expired alpha corpus result = %v, want expired failure", err)
	}
}

func TestSessionFloorRejectsStaleAndConflictingSignedCorpus(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	open := func(serial uint64, target byte) *alpha.Corpus {
		t.Helper()
		raw, issueErr := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: [32]byte{1}, Serial: serial,
			Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{target}}},
			NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, private)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		corpus, openErr := alpha.OpenCorpus(public, raw)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return corpus
	}
	floor, err := alpha.NewSessionFloor("closed-alpha-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(open(2, 9)); err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(open(1, 9)); !alpha.HasFailure(err, alpha.FailureStale) {
		t.Fatalf("stale alpha corpus result = %v, want stale failure", err)
	}
	if err := floor.Observe(open(2, 10)); !alpha.HasFailure(err, alpha.FailureConflict) {
		t.Fatalf("conflicting alpha corpus result = %v, want conflict failure", err)
	}
}

func TestPersistentFloorRetainsCorpusAndRefusesRollbackAfterRestart(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	network := [32]byte{1}
	openCorpus := func(serial uint64, target byte) *alpha.Corpus {
		t.Helper()
		raw, issueErr := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: serial,
			Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{target}}},
			NotBefore: at.Add(-time.Minute), NotAfter: at.Add(time.Hour)}, private)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		corpus, openErr := alpha.OpenCorpus(public, raw)
		if openErr != nil {
			t.Fatal(openErr)
		}
		return corpus
	}
	config := alpha.PersistentFloorConfig{Root: persistentFloorTestRoot(t), Authority: public, Cohort: "closed-alpha-1", Network: network}
	floor, err := alpha.OpenPersistentFloor(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(openCorpus(2, 9)); err != nil {
		t.Fatal(err)
	}
	if err := floor.Close(); err != nil {
		t.Fatal(err)
	}
	floor, err = alpha.OpenPersistentFloor(config)
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	current, err := floor.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current.Serial() != 2 || current.Digest() != openCorpus(2, 9).Digest() {
		t.Fatalf("persistent current corpus = serial %d digest %x", current.Serial(), current.Digest())
	}
	if err := floor.Observe(openCorpus(1, 9)); !alpha.HasFailure(err, alpha.FailureStale) {
		t.Fatalf("persistent stale corpus result = %v", err)
	}
	if err := floor.Observe(openCorpus(2, 10)); !alpha.HasFailure(err, alpha.FailureConflict) {
		t.Fatalf("persistent conflicting corpus result = %v", err)
	}
}

func TestPersistentFloorDoesNotRecoverTemporaryWhileAnotherOpenerOwnsTheLease(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := alpha.PersistentFloorConfig{Root: persistentFloorTestRoot(t), Authority: public, Cohort: "closed-alpha-1", Network: [32]byte{1}}
	floor, err := alpha.OpenPersistentFloor(config)
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	temporary := filepath.Join(config.Root, "corpus-floor.next")
	if err := os.WriteFile(temporary, []byte("active writer fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if other, openErr := alpha.OpenPersistentFloor(config); openErr == nil || other != nil {
		t.Fatalf("concurrent persistent floor open = (%v, %v), want active-root failure", other, openErr)
	}
	if _, err := os.Stat(temporary); err != nil {
		t.Fatalf("concurrent opener removed active temporary successor: %v", err)
	}
}

func persistentFloorTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
