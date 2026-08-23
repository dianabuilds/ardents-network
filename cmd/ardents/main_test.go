package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcceptOfflineCommandPublishesFrozenGeneration(t *testing.T) {
	t.Parallel()
	base := "testdata"
	fixture := t.TempDir()
	inputs := filepath.Join(fixture, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epoch := writeCommandGolden(t, fixture, "epoch.bin", filepath.Join(base, "epoch.hex"))
	material := writeCommandGolden(t, fixture, "material.bin", filepath.Join(base, "materialization-0000.hex"))
	for index := range 8 {
		writeCommandGolden(t, inputs, fmt.Sprintf("%04d.bin", index), filepath.Join(base, fmt.Sprintf("input-%04d.hex", index)))
	}
	stateRoot := filepath.Join(fixture, "state")
	var output bytes.Buffer
	err := run(t.Context(), []string{
		"accept-offline",
		"-state-root", stateRoot,
		"-network-id", "488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d",
		"-authorities", "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631",
		"-threshold", "1",
		"-at", time.Unix(1_800_000_100, 0).UTC().Format(time.RFC3339),
		"-epoch", epoch,
		"-inputs", inputs,
		"-materialization", material,
	}, &output)
	if err != nil {
		t.Fatalf("run accept-offline: %v", err)
	}
	var result struct {
		Generation string `json:"generation"`
		Epoch      uint64 `json:"epoch"`
		ViewLength uint32 `json:"view_length"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result.Generation != "243fba444fe71948f6cd4a253552301192857a156c7eb6359eed604c2d2cda4b" || result.Epoch != 1 || result.ViewLength != 2 {
		t.Fatalf("unexpected command result: %+v", result)
	}
	wantEvent, err := os.ReadFile(filepath.Join(base, "event.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), wantEvent) {
		t.Fatalf("event bytes differ:\n got %s\nwant %s", output.Bytes(), wantEvent)
	}
}

func TestEndpointRouteRejectsIncompleteCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"endpoint", "run"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("incomplete endpoint command err=%v output=%q", err, output.String())
	}
}

func writeCommandGolden(t *testing.T, directory, name, source string) string {
	t.Helper()
	encoded, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
