package stage6evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	workerInputSchema  = "ardents-stage-6-worker-input-v1"
	workerResultSchema = "ardents-stage-6-worker-result-v1"
	maximumWorkerBytes = 4 << 20
)

type workerInput struct {
	Schema          string   `json:"schema"`
	Cell            string   `json:"cell"`
	Ordinal         uint32   `json:"ordinal"`
	Scenario        string   `json:"scenario"`
	ExpectedClass   string   `json:"expected_class"`
	Predicate       string   `json:"predicate"`
	RequiredStreams []string `json:"required_streams"`
	StartOffset     int64    `json:"start_offset_millis"`
	AdmissionSecret [32]byte `json:"admission_secret"`
}

type workerResult struct {
	Schema    string      `json:"schema"`
	Cell      string      `json:"cell"`
	Ordinal   uint32      `json:"ordinal"`
	Class     string      `json:"class"`
	WorkerPID int64       `json:"worker_pid"`
	Trace     traceRecord `json:"trace"`
}

// RunWorker executes exactly one manifest-derived cell within its parent-owned
// staging root. It cannot publish campaign evidence or a verdict.
func RunWorker(root string) error {
	if err := exactWorkerInventory(root, "input.json", "owner"); err != nil {
		return err
	}
	var input workerInput
	if err := readCanonicalWorkerFile(filepath.Join(root, "input.json"), &input); err != nil {
		return err
	}
	if input.Schema != workerInputSchema || int(input.Ordinal) >= len(stage6Cells) || input.StartOffset < 0 ||
		len(input.RequiredStreams) != 1 || input.RequiredStreams[0] != "trace" {
		return errors.New("S6E1 worker input is invalid")
	}
	spec := stage6Cells[input.Ordinal]
	if input.Cell != spec.id || input.Scenario != spec.scenario || input.ExpectedClass != spec.class ||
		input.Predicate != spec.predicate {
		return errors.New("S6E1 worker input does not match the frozen cell")
	}
	trace, class, err := runCell(input.Ordinal, spec, input.AdmissionSecret, input.StartOffset)
	if err != nil {
		return err
	}
	result := workerResult{Schema: workerResultSchema, Cell: spec.id, Ordinal: input.Ordinal,
		Class: class, WorkerPID: int64(os.Getpid()), Trace: trace}
	_, err = writeJSON(root, "result.json", result.Schema, result, false)
	return err
}

func readCanonicalWorkerFile(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > maximumWorkerBytes {
		return errors.New("S6E1 worker file is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("S6E1 worker file is malformed")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("S6E1 worker file is not canonical")
	}
	return nil
}

func exactWorkerInventory(root string, names ...string) error {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("S6E1 worker root is invalid")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(names) {
		return errors.New("S6E1 worker inventory is invalid")
	}
	for index, name := range names {
		if entries[index].Name() != name || !entries[index].Type().IsRegular() || entries[index].Type()&os.ModeSymlink != 0 {
			return errors.New("S6E1 worker inventory is invalid")
		}
	}
	return nil
}
