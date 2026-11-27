package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

type hostingConfig struct {
	Root   string         `json:"root"`
	Policy hosting.Policy `json:"policy"`
}

// decodeHostingConfig validates the exact local budget declaration before effects.
func decodeHostingConfig(raw []byte, operation string) (hostingConfig, error) {
	var p hostingConfig
	bad := errors.New("invalid-input")
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if artifacts.UniqueJSON(d) != nil {
		return p, bad
	}
	if _, err := d.Token(); err != io.EOF {
		return p, bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return p, bad
	}
	want := []string{"root"}
	switch operation {
	case "initialize":
		want = append(want, "policy")
	case "observe":
	default:
		return p, bad
	}
	if len(fields) != len(want) {
		return p, bad
	}
	for _, key := range want {
		value, ok := fields[key]
		if !ok || bytes.Equal(value, []byte("null")) {
			return p, bad
		}
	}
	// encoding/json otherwise matches struct field names case-insensitively,
	// accepting aliases such as provider/Provider as two writes to the same semantic field.
	if operation == "initialize" && !exactHostingObject(fields["policy"], "provider", "start", "end", "unit", "quantity", "directions", "interfaces", "initial_used_bytes", "low_watermark_bytes") {
		return p, bad
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root {
		return p, bad
	}
	return p, nil
}

func exactHostingObject(raw []byte, allowed ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return false
	}
	for name := range fields {
		found := false
		for _, key := range allowed {
			found = found || name == key
		}
		if !found {
			return false
		}
	}
	return true
}
