//go:build live

package network_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const maximumFinalTelemetryFile = 2 << 20

func collectFinalWorkerTelemetry(roots []string) ([]finalRawTelemetry, bool) {
	var result []finalRawTelemetry
	for rootIndex, root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			for _, kind := range []string{"resource.jsonl", "carrier.jsonl"} {
				data, exists, err := readFinalTelemetry(filepath.Join(root, "sync", role, kind))
				if err != nil {
					return nil, false
				}
				if exists {
					result = append(result, finalRawTelemetry{Root: uint16(rootIndex), Role: role, Kind: kind, Data: data})
				}
			}
		}
	}
	return result, true
}

func readFinalTelemetry(path string) ([]byte, bool, error) {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 ||
		before.Size() < 1 || before.Size() > maximumFinalTelemetryFile {
		return nil, false, errors.New("final telemetry is not a bounded regular file")
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	data, readErr := io.ReadAll(io.LimitReader(input, maximumFinalTelemetryFile+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || len(data) > maximumFinalTelemetryFile ||
		!os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, false, errors.Join(readErr, statErr, closeErr, errors.New("final telemetry changed while read"))
	}
	return data, true, nil
}

func TestCollectFinalWorkerTelemetryRetainsRoleStreams(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sync", "bridge")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "resource.jsonl"), []byte("sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, complete := collectFinalWorkerTelemetry([]string{root})
	if !complete || len(files) != 1 || files[0].Role != "bridge" || files[0].Kind != "resource.jsonl" ||
		string(files[0].Data) != "sample\n" {
		t.Fatalf("telemetry=%+v complete=%t", files, complete)
	}
}
