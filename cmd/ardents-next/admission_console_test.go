package main

import "testing"

func TestAdmissionInputRejectsFieldAliasesBeforeOperations(t *testing.T) {
	for _, raw := range []string{
		`{"operation":"status","Operation":"close"}`,
		`{"operation":"status","operation":"close"}`,
		`{"operation":"begin","intent":{"Deadline":"2030-01-01T00:00:00Z","deadline":"2031-01-01T00:00:00Z"}}`,
		`{"operation":"status","payload":null}`,
		`{"unknown":"status"}`,
	} {
		var c holderCommand
		if decodeAdmissionObject([]byte(raw), &c) == nil {
			t.Fatalf("ambiguous input accepted: %s", raw)
		}
	}
	var c holderCommand
	if err := decodeAdmissionObject([]byte(`{"operation":"request","maxima":[0,4,0]}`), &c); err != nil || c.Operation != "request" || c.Maxima[1] != 4 {
		t.Fatal("positive control", err)
	}
}
