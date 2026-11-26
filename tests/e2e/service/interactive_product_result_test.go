//go:build linux || windows

package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"testing"
)

func runCommand(t *testing.T, ctx context.Context, root, binary string, arguments ...string) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = root
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	output, err := command.Output()
	wantDiagnostic := fmt.Sprintf("{\"operation\":%q,\"phase\":\"command\",\"outcome\":\"completed\"}\n", arguments[0]+"."+arguments[1])
	if diagnostic.String() != wantDiagnostic {
		t.Fatalf("command observation: %q", &diagnostic)
	}
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, output)
	}
	return output
}

func runInteractiveProductCommand(t *testing.T, directory, binary string, inputs []interactiveProductInput, arguments ...string) []byte {
	t.Helper()
	output, err := runInteractiveProductCommandResult(t, directory, binary, inputs, arguments...)
	if err != nil {
		t.Fatalf("interactive product command failed: %v\n%s", err, output)
	}
	wantObservation := fmt.Sprintf("{\"operation\":%q,\"phase\":\"command\",\"outcome\":\"completed\"}", "custody."+arguments[0])
	if !bytes.Contains(output, []byte(wantObservation)) {
		t.Fatalf("terminal observation absent: %q", output)
	}
	return output
}

func interactiveProductJSON(t *testing.T, terminal []byte) []byte {
	t.Helper()
	start := bytes.IndexByte(terminal, '{')
	if start < 0 {
		t.Fatalf("interactive product output lacks one JSON receipt: %q", terminal)
	}
	decoder := json.NewDecoder(bytes.NewReader(terminal[start:]))
	var receipt, observation json.RawMessage
	if err := decoder.Decode(&receipt); err != nil {
		t.Fatal("terminal receipt", err)
	}
	if err := decoder.Decode(&observation); err != nil {
		t.Fatal("terminal observation", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected terminal suffix: %v", err)
	}
	return receipt
}
