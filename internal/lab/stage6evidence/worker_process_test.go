package stage6evidence

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "stage6-class-helper" {
		os.Exit(runStage6WorkerClassHelper(os.Args[len(os.Args)-1]))
	}
	os.Exit(m.Run())
}

func TestWorkerProcessReturnsAnObservedClassDifferentFromTheManifest(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input := workerInput{Schema: workerInputSchema, Cell: "A0", Ordinal: 0,
		Scenario: "canonical Service Name", ExpectedClass: "canonical",
		Predicate: "canonical-round-trip", RequiredStreams: []string{"trace"}, StartOffset: 7}
	result, err := executeWorkerProcess(executable,
		[]string{"stage6-class-helper", "-root"}, input)
	if err != nil || result.Class != "behavior-regression" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func runStage6WorkerClassHelper(root string) int {
	var input workerInput
	if err := readCanonicalWorkerFile(filepath.Join(root, "input.json"), &input); err != nil {
		return 1
	}
	result := workerResult{Schema: workerResultSchema, Cell: input.Cell, Ordinal: input.Ordinal,
		Class: "behavior-regression", WorkerPID: int64(os.Getpid()),
		Trace: traceRecord{Schema: "ardents-stage-6-trace-v1", Cell: input.Cell, Ordinal: input.Ordinal,
			Operation: input.Predicate, StartOffset: input.StartOffset}}
	if _, err := writeJSON(root, "result.json", result.Schema, result, false); err != nil {
		return 1
	}
	fmt.Println("complete")
	return 0
}
