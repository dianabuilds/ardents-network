package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification/servicenegative"
)

func TestNegativeCommandObservesEveryRequiredRejection(t *testing.T) {
	var output bytes.Buffer
	if err := run(nil, &output); err != nil {
		t.Fatal(err)
	}
	var value servicenegative.Receipt
	if err := json.Unmarshal(output.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.Schema != "ardents-h3-service-negative-v1" || len(value.Negatives) != 24 {
		t.Fatalf("unexpected negative receipt: %+v", value)
	}
	for name, passed := range value.Negatives {
		if !passed {
			t.Fatalf("negative case %q did not reject", name)
		}
	}
}
