package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

type hostingPlan struct {
	Root        string          `json:"root"`
	Policy      hosting.Policy  `json:"policy"`
	Work        hosting.Traffic `json:"work"`
	Termination hosting.Traffic `json:"termination"`
	HoldMS      uint64          `json:"hold_ms"`
}

// Check every object, including policy and traffic, for duplicate keys before
// typed decoding. Unknown fields and omitted required top-level fields refuse.
func uniqueJSON(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("invalid-input")
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("invalid-input")
			}
			seen[name] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

func decodeHostingPlan(raw []byte, operation string) (hostingPlan, error) {
	var p hostingPlan
	bad := errors.New("invalid-input")
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
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
	case "hold":
		want = append(want, "work", "termination", "hold_ms")
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
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root {
		return p, bad
	}
	if operation == "hold" && (p.HoldMS == 0 || p.HoldMS > 60000) {
		return p, bad
	}
	return p, nil
}
