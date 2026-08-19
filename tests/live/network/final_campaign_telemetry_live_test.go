//go:build live

package network_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maximumFinalTelemetryFile = 2 << 20

func collectFinalWorkerTelemetry(cell string, roots []string) ([]finalRawTelemetry, bool) {
	var result []finalRawTelemetry
	if len(roots) == 1 && (strings.HasPrefix(cell, "capacity/h3-s5-b1-v1/") ||
		strings.HasPrefix(cell, "capacity/h3-s5-b1-v1-strong/")) {
		for _, slot := range finalWorkerTelemetryLayout(cell) {
			rolePath := slot.role
			if slot.role == "endpoint" {
				rolePath = fmt.Sprintf("capacity-%02d", slot.root)
			}
			data, exists, err := readFinalTelemetry(filepath.Join(roots[0], "sync", rolePath, slot.kind))
			if err != nil || !exists {
				return nil, false
			}
			result = append(result, finalRawTelemetry{Root: slot.root, Role: slot.role, Kind: slot.kind, Data: data})
		}
		return result, validFinalWorkerTelemetryInventory(result, cell)
	}
	for _, slot := range finalWorkerTelemetryLayout(cell) {
		if int(slot.root) >= len(roots) {
			return nil, false
		}
		data, exists, err := readFinalTelemetry(filepath.Join(roots[slot.root], "sync", slot.role, slot.kind))
		if err != nil || !exists {
			return nil, false
		}
		result = append(result, finalRawTelemetry{Root: slot.root, Role: slot.role, Kind: slot.kind, Data: data})
	}
	return result, validFinalWorkerTelemetryInventory(result, cell)
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
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		path := filepath.Join(root, "sync", role)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		kinds := []string{"resource.jsonl", "carrier.jsonl", "tree.jsonl"}
		if role == "bridge" {
			kinds = append(kinds, "runtime.jsonl")
		}
		for _, kind := range kinds {
			if err := os.WriteFile(filepath.Join(path, kind), []byte(role+"/"+kind+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	cell := "sustained/endpoint-to-publisher/run-0"
	files, complete := collectFinalWorkerTelemetry(cell, []string{root})
	if !complete || len(files) != 10 || files[7].Role != "publisher" || files[7].Kind != "resource.jsonl" {
		t.Fatalf("telemetry=%+v complete=%t", files, complete)
	}
	if err := os.Remove(filepath.Join(root, "sync", "publisher", "resource.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, complete := collectFinalWorkerTelemetry(cell, []string{root}); complete {
		t.Fatal("incomplete sustained role inventory was accepted")
	}
}

func TestCollectFinalWorkerTelemetryRetainsEveryCapacityEndpoint(t *testing.T) {
	root := t.TempDir()
	for _, role := range []string{"capacity-00", "capacity-01", "capacity-02", "capacity-03", "bridge", "publisher"} {
		path := filepath.Join(root, "sync", role)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		kinds := []string{"resource.jsonl", "carrier.jsonl", "tree.jsonl"}
		if role == "bridge" {
			kinds = append(kinds, "runtime.jsonl")
		}
		for _, kind := range kinds {
			if err := os.WriteFile(filepath.Join(path, kind), []byte(role+"/"+kind+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	cell := "capacity/h3-s5-b1-v1/0"
	files, complete := collectFinalWorkerTelemetry(cell, []string{root})
	if !complete || len(files) != 19 || files[9].Root != 3 || files[12].Role != "bridge" {
		t.Fatalf("capacity telemetry=%+v complete=%t", files, complete)
	}
}
