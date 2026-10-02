package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuanceStrictInput(t *testing.T) {
	base := map[string]any{"root": filepath.Join(t.TempDir(), "keys"), "inventory_file": filepath.Join(t.TempDir(), "public"), "binding": issuanceBindingInput{strings.Repeat("ab", 32), strings.Repeat("cd", 32), strings.Repeat("ef", 32), "2030-01-01T00:00:00Z", "2030-01-01T01:00:00Z"}}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := decodeIssuancePlan(raw); e != nil {
		t.Fatal(e)
	}
	for name, value := range map[string]string{
		"duplicate":  strings.Replace(string(raw), `"network":`, `"network":"`+strings.Repeat("ab", 32)+`","network":`, 1),
		"case":       strings.Replace(string(raw), `"network":`, `"Network":`, 1),
		"missing":    strings.Replace(string(raw), `"signer":"`+strings.Repeat("ef", 32)+`",`, "", 1),
		"unknown":    strings.Replace(string(raw), `"binding":{`, `"binding":{"extra":0,`, 1),
		"uppercase":  strings.Replace(string(raw), strings.Repeat("ab", 32), strings.Repeat("AB", 32), 1),
		"null":       strings.Replace(string(raw), `"binding":{`, `"extra":null,"binding":{`, 1),
		"fractional": strings.Replace(string(raw), "00:00:00Z", "00:00:00.001Z", 1),
		"timezone":   strings.Replace(string(raw), "00:00:00Z", "00:00:00+00:00", 1),
		"trailing":   string(raw) + " {}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeIssuancePlan([]byte(value)); e == nil {
				t.Fatal("accepted")
			}
		})
	}
}
