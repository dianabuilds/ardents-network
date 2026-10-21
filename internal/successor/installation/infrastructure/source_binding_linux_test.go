package infrastructure

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// This fixed persisted-byte vector is separate from the production encoder.
// Its request and rendered-unit commitments use the existing independent
// contract vectors, rather than a binding constructed by Installation.
func sourceBindingVector() []byte {
	unit := strings.ReplaceAll(sourceTemplateOracle, "@ARDENTS_SOURCE_PROGRAM@", `"/var/lib/install/generations/`+strings.Repeat("a", 64)+`/ardents-node-linux-amd64"`)
	unit = strings.ReplaceAll(unit, "@ARDENTS_SOURCE_BINDING@", `"/var/lib/install/binding.json"`)
	unit = strings.ReplaceAll(unit, "@ARDENTS_SOURCE_WRITE_PATHS@", `"/var/lib/state" "/var/lib/participation"`)
	return fmt.Appendf(nil, `{"schema":"ardents-source-binding-v1","invocation":"source","generation":"%s","predecessor":"","request_sha256":"%x","program_target":"ardents/linux-amd64/node","generation_target":"ardents/linux-amd64/infrastructure/source","program_sha256":"%s","template_sha256":"%x","inventory_sha256":"%s","targets_sha256":"%s","plan_sha256":"%s","unit_sha256":"%x","account":"ardents-source","uid":999,"gid":999,"request":%s}`+"\n",
		strings.Repeat("a", 64), sha256.Sum256([]byte(sourceRequestOracle)), strings.Repeat("b", 64), sha256.Sum256([]byte(sourceTemplateOracle)),
		strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), sha256.Sum256([]byte(unit)), strings.TrimSuffix(sourceRequestOracle, "\n"))
}

func TestSourceBindingCanonicalInitialConfiguration(t *testing.T) {
	raw := sourceBindingVector()
	b, err := decodeSourceBinding(raw)
	if err != nil || b.Request.StateRoot != "/var/lib/state" || b.UID != 999 || b.Generation != strings.Repeat("a", 64) {
		t.Fatal("independent initial binding vector refused", err)
	}
	for _, change := range []struct{ name, before, after string }{
		{"schema", `ardents-source-binding-v1`, `ardents-source-binding-v2`},
		{"role", `"invocation":"source"`, `"invocation":"forwarding"`},
		{"duplicate", `"predecessor":""`, `"predecessor":"","predecessor":""`},
		{"successor", `"predecessor":""`, `"predecessor":"` + strings.Repeat("f", 64) + `"`},
		{"unknown", `"uid":999`, `"extra":true,"uid":999`},
		{"order", `"uid":999,"gid":999`, `"gid":999,"uid":999`},
		{"root", `"uid":999`, `"uid":0`},
		{"group", `"gid":999`, `"gid":998`},
		{"account", `"account":"ardents-source"`, `"account":"ardents-node"`},
		{"endpoint-program", `ardents/linux-amd64/node`, `ardents/linux-amd64/endpoint`},
		{"endpoint-generation", `ardents/linux-amd64/infrastructure/source`, `ardents/linux-amd64/protected-endpoint`},
		{"generation-case", strings.Repeat("a", 64), strings.Repeat("A", 64)},
		{"generation-unit-mismatch", strings.Repeat("a", 64), strings.Repeat("f", 64)},
		{"request-commitment", `/var/lib/state`, `/var/lib/foreign-state`},
		{"request-grammar", `"profile":"ardents-source-process-v1"`, `"profile":"endpoint"`},
		{"short-digest", strings.Repeat("b", 64), "ab"},
		{"nonhex", strings.Repeat("b", 64), strings.Repeat("g", 64)},
		{"missing", `"plan_sha256":"` + strings.Repeat("e", 64) + `",`, ""},
	} {
		t.Run(change.name, func(t *testing.T) {
			altered := strings.Replace(string(raw), change.before, change.after, 1)
			if altered == string(raw) {
				t.Fatal("control failed to alter vector")
			}
			if got, err := decodeSourceBinding([]byte(altered)); err == nil || got.Schema != "" {
				t.Fatal("substituted/ambiguous binding admitted", err)
			}
		})
	}
	for _, malformed := range [][]byte{nil, raw[:len(raw)-1], append(append([]byte(nil), raw...), '\n'), []byte(strings.Repeat(" ", 32<<10+1))} {
		if _, err := decodeSourceBinding(malformed); err == nil {
			t.Fatal("unbounded/noncanonical binding accepted")
		}
	}
}
