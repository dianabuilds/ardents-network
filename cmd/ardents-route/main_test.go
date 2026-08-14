package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestCommandErrorIsStructuredForEvidenceCollectors(t *testing.T) {
	var output bytes.Buffer
	cause := errors.New("authenticate initiator: expected failure")
	if err := writeCommandError(&output, cause); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Kind, Error string
	}
	if err := json.Unmarshal(output.Bytes(), &value); err != nil {
		t.Fatalf("command error is not structured JSON: %v", err)
	}
	if value.Kind != "error" || value.Error != cause.Error() {
		t.Fatalf("command error = %+v", value)
	}
}

func TestCommandErrorPreservesWriteFailure(t *testing.T) {
	cause := errors.New("write failed")
	err := writeCommandError(errorWriter{err: cause}, errors.New("Route failed"))
	if !errors.Is(err, cause) || err.Error() != "write command error diagnostic: write failed" {
		t.Fatalf("write error = %v; want contextualized %v", err, cause)
	}
}

type errorWriter struct{ err error }

func (writer errorWriter) Write([]byte) (int, error) { return 0, writer.err }

var _ io.Writer = errorWriter{}
