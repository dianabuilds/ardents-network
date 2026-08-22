package updatetransaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestRepeatedUpdatesKeepRetainedStateBounded exercises the retained-success
// path 100 times. The assertion is intentionally based on the on-disk parent
// directory rather than private inspection helpers: a successful successor
// must first retire its old rollback and transaction before it may publish a
// new pair.
func TestRepeatedUpdatesKeepRetainedStateBounded(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	var finalMeasurements []pressureMeasurement
	for generation := uint64(1); generation <= 100; generation++ {
		request := Request{UpdateRoot: root, Generation: generation, SchemaPlan: "no-op-v1",
			decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
		result, err := Apply(context.Background(), request)
		if err != nil || result.Outcome != "committed" || result.Generation != generation || result.StagingPresent {
			t.Fatalf("episode %d Apply = %+v, %v", generation, result, err)
		}
		measurement := measureRetainedRoot(t, root, generation)
		if generation > 80 {
			finalMeasurements = append(finalMeasurements, measurement)
		}
	}
	if len(finalMeasurements) != 20 {
		t.Fatalf("final measurements = %d, want 20", len(finalMeasurements))
	}
	for index, measurement := range finalMeasurements[1:] {
		if measurement != finalMeasurements[0] {
			t.Fatalf("final episode %d measurement = %+v, baseline = %+v", index+82, measurement, finalMeasurements[0])
		}
	}
}

// TestPressureRefusalsAndRecoveryCycle keeps destructive terminal paths
// isolated, while repeatedly proving bounded refusal cleanup and idempotent
// restart recovery. A recovered journal remains an auditable terminal record;
// a fresh update uses its own next lifecycle rather than overwriting it.
func TestPressureRefusalsAndRecoveryCycle(t *testing.T) {
	for episode := 0; episode < 100; episode++ {
		root := filepath.Join(t.TempDir(), "update")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		vector := oracleBootstrapV0(t, root)
		candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
		request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
			decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
		switch episode % 6 {
		case 0:
			work := &drainRefusalWorkControl{stopErr: errors.New("pressure stop refusal")}
			request.Work = work
			result, err := Apply(context.Background(), request)
			if err == nil || result.Outcome != "drain-expired" || work.stopCalls != 1 || work.drainCalls != 0 {
				t.Fatalf("episode %d stop refusal = %+v, %v; work=%+v", episode, result, err, work)
			}
		case 1:
			control := &applyInterruptionControl{StopBefore: func(name string) bool { return name == "03-staged" }}
			if result, err := applyWithInterruption(context.Background(), request, control); !errors.Is(err, errApplyInterrupted) || result != (Result{}) {
				t.Fatalf("episode %d interruption = %+v, %v", episode, result, err)
			}
			first, recoverErr := Recover(context.Background(), root)
			second, repeatErr := Recover(context.Background(), root)
			if recoverErr != nil || repeatErr != nil || first != second {
				t.Fatalf("episode %d recovery = %+v, %v; repeat = %+v, %v", episode, first, recoverErr, second, repeatErr)
			}
			continue
		case 2:
			result, err := Apply(context.Background(), request)
			if err != nil || result.Outcome != "committed" {
				t.Fatalf("episode %d success = %+v, %v", episode, result, err)
			}
			continue
		case 3:
			request.SelfTest = failedSelfTest{}
			if result, err := Apply(context.Background(), request); err == nil || result.Outcome != "self-test-failed" || result.State != "rollback-pending" {
				t.Fatalf("episode %d failed self-test = %+v, %v", episode, result, err)
			}
			request.rollbackDecision = oracleRollbackDecision(t, vector)
			request.SelfTest = oraclePassSelfTest{}
			result, err := Apply(context.Background(), request)
			if !errors.Is(err, errRolledBack) || result.Outcome != "rolled-back" || result.State != "rolled-back" {
				t.Fatalf("episode %d rollback = %+v, %v", episode, result, err)
			}
			continue
		case 4:
			request.SelfTest = failedSelfTest{}
			if result, err := Apply(context.Background(), request); err == nil || result.Outcome != "self-test-failed" {
				t.Fatalf("episode %d failed self-test = %+v, %v", episode, result, err)
			}
			request.rollbackDecision = request.decision
			result, err := Apply(context.Background(), request)
			if !errors.Is(err, errRollbackRefused) || result.Outcome != "rollback-refused" || result.State != "repair-required" {
				t.Fatalf("episode %d rollback refusal = %+v, %v", episode, result, err)
			}
			continue
		case 5:
			request.SelfTest = failedSelfTest{}
			if result, err := Apply(context.Background(), request); err == nil || result.Outcome != "self-test-failed" {
				t.Fatalf("episode %d failed self-test = %+v, %v", episode, result, err)
			}
			request.rollbackDecision = oracleRollbackDecision(t, vector)
			result, err := Apply(context.Background(), request)
			if !errors.Is(err, errRepairRequired) || result.Outcome != "repair-required" || result.State != "repair-required" {
				t.Fatalf("episode %d repair required = %+v, %v", episode, result, err)
			}
			continue
		}
		request.Work = &oracleWorkControl{}
		result, err := Apply(context.Background(), request)
		if err != nil || result.Outcome != "committed" {
			t.Fatalf("episode %d fresh update = %+v, %v", episode, result, err)
		}
		measureRetainedRoot(t, root, request.Generation)
	}
}

// TestPressure100 follows the accepted S7.2-08a twenty-row oracle five times.
func TestPressure100(t *testing.T) {
	codes := []byte{'S', 'P', 'I', 'R', 'B', 'F', 'Q', 'N', 'P', 'I', 'R', 'B', 'F', 'Q', 'N', 'P', 'I', 'R', 'P', 'S'}
	for block := 0; block < 5; block++ {
		var interrupted, refused string
		for row, code := range codes {
			root := filepath.Join(t.TempDir(), "root-"+strconv.Itoa(block)+"-"+strconv.Itoa(row+1))
			var request Request
			var vector v0OracleVector
			if code == 'R' {
				root = interrupted
			} else if code == 'Q' {
				root = refused
			} else {
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				vector = oracleBootstrapV0(t, root)
				candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
				request = Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1", decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
			}
			switch code {
			case 'S', 'N':
				result, err := Apply(context.Background(), request)
				if err != nil || result.Outcome != "committed" || result.State != "committed" {
					t.Fatalf("%d/%d %c = %+v, %v", block, row+1, code, result, err)
				}
			case 'P':
				request.decision.Length = maximumArtifactBytes + 1
				result, err := Apply(context.Background(), request)
				if err == nil || result.Outcome != "resource-denied" || result.State != "release-accepted" {
					t.Fatalf("%d/%d P = %+v, %v", block, row+1, result, err)
				}
			case 'I':
				interrupted = root
				control := &applyInterruptionControl{StopAfter: func(name string) bool { return name == "03-staged" }}
				if result, err := applyWithInterruption(context.Background(), request, control); !errors.Is(err, errApplyInterrupted) || result != (Result{}) {
					t.Fatalf("%d/%d I = %+v, %v", block, row+1, result, err)
				}
			case 'R':
				result, err := Recover(context.Background(), root)
				if err != nil || result.Outcome != "recovered" || result.State != "staged" {
					t.Fatalf("%d/%d R = %+v, %v", block, row+1, result, err)
				}
			case 'B', 'F':
				request.SelfTest = failedSelfTest{}
				if _, err := Apply(context.Background(), request); err == nil {
					t.Fatalf("%d/%d %c self-test succeeded", block, row+1, code)
				}
				if code == 'B' {
					request.rollbackDecision = oracleRollbackDecision(t, vector)
					request.SelfTest = oraclePassSelfTest{}
				} else {
					request.rollbackDecision = request.decision
					refused = root
				}
				result, err := Apply(context.Background(), request)
				if code == 'B' && (!errors.Is(err, errRolledBack) || result.Outcome != "rolled-back") {
					t.Fatalf("%d/%d B = %+v, %v", block, row+1, result, err)
				}
				if code == 'F' && (!errors.Is(err, errRollbackRefused) || result.Outcome != "rollback-refused") {
					t.Fatalf("%d/%d F = %+v, %v", block, row+1, result, err)
				}
			case 'Q':
				result, err := Recover(context.Background(), root)
				if !errors.Is(err, errRollbackRefused) || result.Outcome != "rollback-refused" {
					t.Fatalf("%d/%d Q = %+v, %v", block, row+1, result, err)
				}
			}
		}
	}
}

type pressureMeasurement struct{ Files, Directories, Bytes int }

func measureRetainedRoot(t *testing.T, root string, generation uint64) pressureMeasurement {
	t.Helper()
	staging, err := os.ReadDir(filepath.Join(root, "staging"))
	if err != nil || len(staging) != 0 {
		t.Fatalf("episode %d staging = %v, %v", generation, staging, err)
	}
	if _, err := os.Lstat(filepath.Join(root, rollbackRetireName)); !os.IsNotExist(err) {
		t.Fatalf("episode %d retained %s: %v", generation, rollbackRetireName, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "generations"))
	if err != nil {
		t.Fatal(err)
	}
	wantGenerations := []string{strconv.FormatUint(generation-1, 10), strconv.FormatUint(generation, 10)}
	sort.Strings(wantGenerations)
	gotGenerations := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotGenerations = append(gotGenerations, entry.Name())
	}
	if !reflect.DeepEqual(gotGenerations, wantGenerations) {
		t.Fatalf("episode %d generations = %v, want %v", generation, gotGenerations, wantGenerations)
	}
	transactions, err := os.ReadDir(filepath.Join(root, "transactions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(transactions) != 1 || transactions[0].Name() != strconv.FormatUint(generation, 10) {
		t.Fatalf("episode %d transactions = %v", generation, transactions)
	}
	selectionRaw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeCurrent(selectionRaw)
	if err != nil || selection.Transaction != generation || selection.Current.Generation != generation ||
		selection.Rollback == nil || selection.Rollback.Generation != generation-1 {
		t.Fatalf("episode %d selection = %+v, %v", generation, selection, err)
	}
	measurement := pressureMeasurement{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			measurement.Directories++
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".current.") || strings.HasPrefix(entry.Name(), ".schema-current.") {
			return os.ErrInvalid
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		measurement.Files++
		measurement.Bytes += int(info.Size())
		return nil
	})
	if err != nil {
		t.Fatalf("episode %d retained root walk: %v", generation, err)
	}
	return measurement
}
