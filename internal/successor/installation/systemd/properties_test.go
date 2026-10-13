package systemd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestManagerJSONRefusesDuplicateVariantsUnknownFieldsAndTrailingData(t *testing.T) {
	for _, raw := range []string{
		`{"type":"b","type":"b","data":true}`, `{"type":"b","data":true,"foreign":0}`, `{"type":"b","data":true} {}`,
		`{"type":"a{sv}","data":[{"NoNewPrivileges":{"type":"b","data":false},"NoNewPrivileges":{"type":"b","data":true}}]}`,
	} {
		var value Value
		if err := Decode([]byte(raw), &value); !errors.Is(err, ErrObservation) {
			t.Fatal("ambiguous manager response accepted", err)
		}
	}
	var payload []Properties
	if err := Decode([]byte(`[{"NoNewPrivileges":{"type":"b","data":true,"foreign":0}}]`), &payload); !errors.Is(err, ErrObservation) {
		t.Fatal("unknown nested variant field accepted", err)
	}
	for _, raw := range []string{`0.0`, `null`, `false`, `18446744073709551615`} {
		properties := Properties{"CapabilityBoundingSet": {Type: "t", Data: json.RawMessage(raw)}}
		if Matches(properties, "CapabilityBoundingSet", "t", uint64(0)) {
			t.Fatal("nonzero/untyped representation accepted", raw)
		}
	}
}

func TestManagerPropertyMatchPreservesRepresentationAndBounds(t *testing.T) {
	for _, test := range []struct {
		name, raw string
		wanted    any
		match     bool
	}{
		{"boolean", `true`, true, true},
		{"integer precision", `18446744073709551615`, ^uint64(0), true},
		{"whitespace", ` [ "reader", "publisher" ] `, []string{"reader", "publisher"}, true},
		{"escaped spelling", `"\u0072eader"`, "reader", true},
		{"duplicate key", `{"x":true,"x":true}`, map[string]bool{"x": true}, false},
		{"numeric spelling", `1.0`, 1, false},
		{"trailing data", `true false`, true, false},
		{"different value", `false`, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			properties := Properties{"value": {Type: "test", Data: json.RawMessage(test.raw)}}
			if got := Matches(properties, "value", "test", test.wanted); got != test.match {
				t.Fatalf("match=%v want %v", got, test.match)
			}
			if Matches(properties, "value", "foreign", test.wanted) || Matches(properties, "absent", "test", test.wanted) {
				t.Fatal("missing or differently typed property accepted")
			}
		})
	}
	// Equal bytes cannot bypass the original parser's input or nesting bounds.
	for _, raw := range []string{`"` + strings.Repeat("x", 64<<10) + `"`, strings.Repeat("[", 18) + `0` + strings.Repeat("]", 18)} {
		properties := Properties{"value": {Type: "test", Data: json.RawMessage(raw)}}
		if Matches(properties, "value", "test", json.RawMessage(raw)) {
			t.Fatal("identical out-of-bound value accepted")
		}
	}
}

func BenchmarkManagerPropertyMatch(b *testing.B) {
	wanted := []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_SETGID", "CAP_SETUID"}
	raw, err := json.Marshal(wanted)
	if err != nil {
		b.Fatal(err)
	}
	properties := Properties{"CapabilitySet": {Type: "as", Data: raw}}
	b.ReportAllocs()
	for b.Loop() {
		if !Matches(properties, "CapabilitySet", "as", wanted) {
			b.Fatal("matching observed property refused")
		}
	}
}
