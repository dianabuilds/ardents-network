package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type applyCheckpointState struct {
	journal, current                int
	staging, published, currentTemp bool
}

type applyCheckpointEvent struct {
	name          string
	before, after applyCheckpointState
}

func TestApplyInterruptedPrefixes(t *testing.T) {
	events := []applyCheckpointEvent{
		{"01-release-accepted", applyCheckpointState{}, applyCheckpointState{journal: 1}},
		{"02-artifact-verified", applyCheckpointState{journal: 1}, applyCheckpointState{journal: 2}},
		{"03-staged", applyCheckpointState{journal: 2, staging: true}, applyCheckpointState{journal: 3, staging: true}},
		{"04-rollback-reserved", applyCheckpointState{journal: 3, staging: true}, applyCheckpointState{journal: 4, staging: true}},
		{"05-stop-new-work", applyCheckpointState{journal: 4, staging: true}, applyCheckpointState{journal: 5, staging: true}},
		{"06-draining", applyCheckpointState{journal: 5, staging: true}, applyCheckpointState{journal: 6, staging: true}},
		{"publish-generation", applyCheckpointState{journal: 6, staging: true}, applyCheckpointState{journal: 6, published: true}},
		{"current-temp", applyCheckpointState{journal: 6, published: true}, applyCheckpointState{journal: 6, published: true, currentTemp: true}},
		{"replace-current", applyCheckpointState{journal: 6, published: true, currentTemp: true}, applyCheckpointState{journal: 6, current: 1, published: true}},
		{"durability-ack", applyCheckpointState{journal: 6, current: 1, published: true}, applyCheckpointState{journal: 6, current: 1, published: true}},
		{"07-activated", applyCheckpointState{journal: 6, current: 1, published: true}, applyCheckpointState{journal: 7, current: 1, published: true}},
		{"08-self-testing", applyCheckpointState{journal: 7, current: 1, published: true}, applyCheckpointState{journal: 8, current: 1, published: true}},
		{"09-committed", applyCheckpointState{journal: 8, current: 1, published: true}, applyCheckpointState{journal: 9, current: 1, published: true}},
	}
	for _, event := range events {
		for _, phase := range []struct {
			name   string
			before bool
			want   applyCheckpointState
		}{{"before", true, event.before}, {"after", false, event.after}} {
			event, phase := event, phase
			t.Run(event.name+"/"+phase.name, func(t *testing.T) {
				root, request := applyCheckpointRequest(t)
				calls := 0
				stop := func(name string) bool {
					if name != event.name {
						return false
					}
					calls++
					return true
				}
				control := &applyInterruptionControl{}
				if phase.before {
					control.StopBefore = stop
				} else {
					control.StopAfter = stop
				}
				result, err := applyWithInterruption(context.Background(), request, control)
				if !errors.Is(err, errApplyInterrupted) {
					t.Fatalf("Apply error=%v, want interruption sentinel", err)
				}
				if result != (Result{}) {
					t.Fatalf("interrupted Apply constructed Result: %+v", result)
				}
				if calls != 1 {
					t.Fatalf("checkpoint calls=%d, want 1", calls)
				}
				assertApplyCheckpointState(t, root, phase.want)
				lock, lockErr := acquireOwnedLock(root)
				if lockErr != nil {
					t.Fatalf("outer Apply did not release OS lock: %v", lockErr)
				}
				if releaseErr := lock.release(); releaseErr != nil {
					t.Fatalf("release probe lock: %v", releaseErr)
				}
			})
		}
	}
}

func applyCheckpointRequest(t *testing.T) (string, Request) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{
		UpdateRoot: root, generation: vector.Request.TransactionGeneration,
		schemaPlan: vector.Request.SchemaPlan,
		decision:   oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{},
	}
	if _, _, _, err := validateRequest(context.Background(), request); err != nil {
		t.Fatalf("checkpoint fixture request: %v", err)
	}
	return root, request
}

func assertApplyCheckpointState(t *testing.T, root string, want applyCheckpointState) {
	t.Helper()
	journalRoot := filepath.Join(root, "transactions", "1", "journal")
	entries, err := os.ReadDir(journalRoot)
	if err != nil {
		t.Fatalf("read journal prefix: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	allNames := []string{
		"01-release-accepted.entry", "02-artifact-verified.entry", "03-staged.entry",
		"04-rollback-reserved.entry", "05-stop-new-work.entry", "06-draining.entry",
		"07-activated.entry", "08-self-testing.entry", "09-committed.entry",
	}
	if !reflect.DeepEqual(names, allNames[:want.journal]) {
		t.Fatalf("journal prefix=%v, want %v", names, allNames[:want.journal])
	}
	assertCheckpointPath(t, filepath.Join(root, "staging", "1"), want.staging)
	assertCheckpointPath(t, filepath.Join(root, "generations", "1"), want.published)
	currentRaw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeCurrent(currentRaw)
	if err != nil || int(selection.Transaction) != want.current {
		t.Fatalf("current transaction=%d err=%v, want %d", selection.Transaction, err, want.current)
	}
	rootEntries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	temps := 0
	for _, entry := range rootEntries {
		if strings.HasPrefix(entry.Name(), ".current.") && strings.HasSuffix(entry.Name(), ".tmp") {
			temps++
		}
	}
	if (temps == 1) != want.currentTemp {
		t.Fatalf("current temp count=%d, want present=%t", temps, want.currentTemp)
	}
	lockBytes, err := os.ReadFile(filepath.Join(root, lockFileName))
	if err != nil || len(lockBytes) != 0 {
		t.Fatalf("permanent lock bytes=%x err=%v", lockBytes, err)
	}
}

func assertCheckpointPath(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Lstat(path)
	present := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if present != want {
		t.Fatalf("path %s present=%t, want %t", path, present, want)
	}
}
