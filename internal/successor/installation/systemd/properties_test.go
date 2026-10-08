package systemd

import (
	"encoding/json"
	"errors"
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
