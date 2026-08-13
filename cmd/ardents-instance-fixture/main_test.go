package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProcessReturnsOnlyPublicInstanceMaterial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.hex")
	var output bytes.Buffer
	if err := run([]string{"generate", path}, &output); err != nil {
		t.Fatal(err)
	}
	var receipt publicReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Schema != "ardents-h3-instance-host-v1" || len(receipt.Public) != 64 || len(private) != 128 ||
		bytes.Contains(output.Bytes(), private) {
		t.Fatalf("host fixture crossed the custody boundary: receipt=%+v private-bytes=%d", receipt, len(private))
	}
	if err := run([]string{"generate", path}, &output); err == nil {
		t.Fatal("host fixture overwrote an existing Instance key")
	}
}
