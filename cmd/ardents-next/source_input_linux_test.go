//go:build linux

package main

import (
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"testing"
)

func TestSourcePlanRefusesAmbiguousOrMixedDeclarations(t *testing.T) {
	plan := source.ServingDeclaration{Network: &source.ServingNetwork{NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, ProfileAuthority: [32]byte{2}}, ClientPins: [][32]byte{{3}}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.DecodeServingDeclaration(raw); err != nil {
		t.Fatal("complete canonical shape refused", err)
	}
	for _, fault := range []string{"short-pin", "long-pin", "short-authority", "unknown", "mixed", "duplicate", "trailing"} {
		t.Run(fault, func(t *testing.T) {
			var object map[string]any
			if err := json.Unmarshal(raw, &object); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "short-pin":
				object["client_pins"] = []any{[31]byte{3}}
			case "long-pin":
				object["client_pins"] = []any{[33]byte{3}}
			case "short-authority":
				object["network"].(map[string]any)["authorities"] = []any{[31]byte{2}}
			case "unknown":
				object["verification_clock"] = "other-clock"
			case "mixed":
				object["sources"] = []any{}
			}
			changed, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "duplicate" {
				changed = append(append([]byte(`{"address":"other",`), raw[1:]...), '\n')
			}
			if fault == "trailing" {
				changed = append(changed, []byte(" {}")...)
			}
			if _, err := source.DecodeServingDeclaration(changed); err == nil {
				t.Fatal("ambiguous Source declaration accepted")
			}
		})
	}
}
