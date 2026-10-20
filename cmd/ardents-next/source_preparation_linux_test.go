//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSourcePreparationClosedDeclaration(t *testing.T) {
	p, _ := sourcePreparationFixture(t, t.TempDir(), installedHeadless{})
	healthy := sourcePreparationJSON(t, p)
	if _, _, err := decodeSourcePreparation(healthy); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing", "unknown", "duplicate", "trailing", "short-id", "long-id", "mixed", "incomplete-member", "three-sources", "null", "fractional", "clock-override"} {
		t.Run(fault, func(t *testing.T) {
			var q map[string]any
			if json.Unmarshal(healthy, &q) != nil {
				t.Fatal("fixture")
			}
			var raw []byte
			switch fault {
			case "missing":
				delete(q, "client_private_key")
			case "unknown":
				q["issuer"] = true
			case "duplicate":
				raw = append([]byte(`{"schema":"wrong",`), healthy[1:]...)
			case "trailing":
				raw = append(bytes.Clone(healthy), []byte("{}")...)
			case "short-id":
				q["client_leaf_key_digest"] = "01"
			case "long-id":
				q["client_leaf_key_digest"] = q["client_leaf_key_digest"].(string) + "00"
			case "mixed":
				q["sources"].([]any)[0].(map[string]any)["private_bytes"] = "forbidden"
			case "incomplete-member":
				delete(q["sources"].([]any)[0].(map[string]any), "root")
			case "three-sources":
				q["sources"] = append(q["sources"].([]any), q["sources"].([]any)[0])
			case "null":
				q["client_root"] = nil
			case "fractional":
				q["materialization_index"] = 0.5
			case "clock-override":
				q["client"].(map[string]any)["verification_clock"] = "override"
			}
			if raw == nil {
				raw = sourcePreparationJSON(t, q)
			}
			if _, _, err := decodeSourcePreparation(raw); err == nil {
				t.Fatal("closed declaration accepted", fault)
			}
		})
	}
}
