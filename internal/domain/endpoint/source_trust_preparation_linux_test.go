//go:build linux

package endpoint

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSourcePreparationClosedDeclaration(t *testing.T) {
	identity := strings.Repeat("ab", 32)
	public := map[string]any{"path": "/cert.pem", "sha256": identity}
	member := map[string]any{
		"identity": identity, "family": "fixture", "endpoint_handle": "fixture",
		"address": "127.0.0.1:8040", "server_name": "fixture.test",
		"certificate": public, "root": public, "leaf_key_digest": identity,
		"private_key": "/private.pem", "state_root": "/state", "participation_root": "/participation",
	}
	healthy, err := json.Marshal(map[string]any{
		"schema":         "ardents-source-trust-preparation-v1",
		"reference_time": "2026-11-23T00:00:00Z", "clock_observed_at": "2026-11-23T00:00:00Z",
		"client":             HeadlessPlan{Schema: "ardents-headless-runtime-v2", NetworkAuthorities: []string{identity}},
		"client_certificate": public, "client_root": public, "client_leaf_key_digest": identity, "client_private_key": "/client.pem",
		"order_seed": identity, "materialization_index": 0, "refresh_interval_ms": 1000,
		"sources": []map[string]any{member, member},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := decodeSourcePreparation(healthy); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing", "unknown", "duplicate", "duplicate-nested", "short-maxima", "trailing", "short-id", "long-id", "mixed", "incomplete-member", "three-sources", "null", "fractional", "clock-override"} {
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
			case "duplicate-nested":
				raw = bytes.Replace(healthy, []byte(`"maxima":[0,0,0]`), []byte(`"maxima":[0,0,0],"maxima":[0,0,0]`), 1)
			case "short-maxima":
				q["client"].(map[string]any)["reader_permission"].(map[string]any)["maxima"] = []uint32{0, 0}
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
				var err error
				raw, err = json.Marshal(q)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := decodeSourcePreparation(raw); err == nil {
				t.Fatal("closed declaration accepted", fault)
			}
		})
	}
}
