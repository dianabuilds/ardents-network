package main

import (
	"bytes"
	"testing"
)

func TestEncodeNameCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := runName([]string{"encode", "alice"}, &output); err != nil {
		t.Fatalf("run: %v", err)
	}
	if output.String() != "000105616c696365\n" {
		t.Fatalf("encoded name = %q", output.String())
	}
	if err := runName([]string{"encode", "Alice"}, &output); err == nil {
		t.Fatal("command accepted a non-canonical name")
	}
}
