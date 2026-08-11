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

	"github.com/dianabuilds/ardents-network/internal/qualification"
)

func TestOfflineCommandRendersIndependentPass(t *testing.T) {
	t.Parallel()
	const generation = "243fba444fe71948f6cd4a253552301192857a156c7eb6359eed604c2d2cda4b"
	base := filepath.Join("..", "..", "tests", "qualification", "h3-s1-offline-v1", "testdata")
	root := t.TempDir()
	inputs := filepath.Join(root, "generations", generation, "inputs")
	if err := os.MkdirAll(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	writeQualifierGolden(t, filepath.Join(root, "generations", generation, "epoch.bin"), filepath.Join(base, "epoch.hex"))
	for index := range 8 {
		writeQualifierGolden(t, filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), filepath.Join(base, fmt.Sprintf("input-%04d.hex", index)))
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(generation+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	material := filepath.Join(t.TempDir(), "material.bin")
	writeQualifierGolden(t, material, filepath.Join(base, "materialization-0000.hex"))
	var output, diagnostics bytes.Buffer
	code := run([]string{
		"offline", "-state-root", root,
		"-network-id", "488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d",
		"-authorities", "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631",
		"-threshold", "1",
		"-at", time.Unix(1_800_000_100, 0).UTC().Format(time.RFC3339),
		"-materializations", material,
	}, &output, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("exit=%d diagnostics=%q", code, diagnostics.String())
	}
	var result qualification.Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "pass" || result.Generation != generation {
		t.Fatalf("unexpected result: %+v", result)
	}
	wantVerdict, err := os.ReadFile(filepath.Join(base, "verdict.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), wantVerdict) {
		t.Fatalf("verdict bytes differ:\n got %s\nwant %s", output.Bytes(), wantVerdict)
	}
}

func TestOfflineCommandUsesInvalidExitForBadInvocation(t *testing.T) {
	t.Parallel()
	var output, diagnostics bytes.Buffer
	if code := run(nil, &output, &diagnostics); code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	var result qualification.Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "invalid" {
		t.Fatalf("verdict=%q, want invalid", result.Verdict)
	}
}

func writeQualifierGolden(t *testing.T, destination, source string) {
	t.Helper()
	encoded, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
