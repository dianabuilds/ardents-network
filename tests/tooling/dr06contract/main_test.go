package main

import (
	"strings"
	"testing"
)

func TestValidateRejectsInvalidYAML(t *testing.T) {
	t.Parallel()
	err := validate([]byte("jobs: ["), nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "YAML is invalid") {
		t.Fatalf("validate() error = %v, want YAML failure", err)
	}
}

func TestValidateRejectsMissingQualificationInputs(t *testing.T) {
	t.Parallel()
	err := validate([]byte("name: CI\njobs: {}\n"), []byte(`{"schema_version":1,"gates":[]}`), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "17 gates") {
		t.Fatalf("validate() error = %v, want incomplete gate contract", err)
	}
}
