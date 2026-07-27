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

func TestValidateCapabilityGateOwnershipRejectsDivergentJob(t *testing.T) {
	t.Parallel()
	contract := []byte(`{
		"schema_version": 1,
		"gates": [{
			"id": "application-process",
			"workflow_job_id": "focused-tagged",
			"command": "./ardents.ps1 test e2e -Scenario APP-001"
		}]
	}`)
	capabilities := []byte(`{
		"evidence_gates": [{
			"id": "application-process",
			"ci_job": "tagged",
			"scenario_id": "APP-001"
		}]
	}`)
	err := validateCapabilityGateOwnership(contract, capabilities)
	if err == nil || !strings.Contains(err.Error(), "owned by CI job tagged") {
		t.Fatalf("validateCapabilityGateOwnership() error = %v, want owner mismatch", err)
	}
}
