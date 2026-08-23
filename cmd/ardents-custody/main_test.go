package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectEnvelopeRejectsMissingInputsBeforeCreatingCustodyState(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"inspect-envelope"}, &output); err == nil {
		t.Fatal("inspect accepted missing inputs")
	}
}

func TestInspectEnvelopeRendersOnlyPublicHeaderFacts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "envelope.json")
	body := []byte(`{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"recovery-bundle","kdf":{"name":"argon2id","version":19,"memory_kib":262144,"passes":3,"lanes":4,"salt":"AAAAAAAAAAAAAAAAAAAAAA"},"aead":"aes-256-gcm-random-nonce","ciphertext":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(t.Context(), []string{"inspect-envelope", "-vault-root", filepath.Join(root, "vault"), "-envelope", path}, &output); err != nil {
		t.Fatalf("inspect envelope: %v", err)
	}
	var result struct {
		Schema    string `json:"schema"`
		Purpose   string `json:"purpose"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != "ardents-custody-inspection-v1" || result.Purpose != "recovery-bundle" || result.Operation != "inspect-envelope" {
		t.Fatalf("unexpected inspection result: %+v", result)
	}
}
