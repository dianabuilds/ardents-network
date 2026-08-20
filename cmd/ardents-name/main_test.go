package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func TestEncodeNameCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run([]string{"encode-name", "alice"}, &output); err != nil {
		t.Fatalf("run: %v", err)
	}
	if output.String() != "000105616c696365\n" {
		t.Fatalf("encoded name = %q", output.String())
	}
	if err := run([]string{"encode-name", "Alice"}, &output); err == nil {
		t.Fatal("command accepted a non-canonical name")
	}
}

func TestValidateRecordCommand(t *testing.T) {
	t.Parallel()
	record := namelease.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: "authority", LeaseExpiresAt: 200, GraceExpiresAt: 220}
	wire, err := namelease.EncodeRecord(record)
	if err != nil {
		t.Fatalf("EncodeRecord: %v", err)
	}
	path := filepath.Join(t.TempDir(), "record.bin")
	if err := os.WriteFile(path, wire, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"validate-record", path}, &output); err != nil {
		t.Fatalf("run: %v", err)
	}
	if output.String() != "valid\n" {
		t.Fatalf("validation output = %q", output.String())
	}
	if err := os.WriteFile(path, append(wire, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"validate-record", path}, &output); err == nil {
		t.Fatal("command accepted a non-canonical record")
	}
}
