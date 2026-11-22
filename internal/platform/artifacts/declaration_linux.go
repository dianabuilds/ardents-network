package artifacts

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"
)

// DecodeDeclaration requires exactly the named non-null JSON fields.
func DecodeDeclaration(raw []byte, value any, fields ...string) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return ErrInput
	}
	wanted, seen := map[string]bool{}, map[string]bool{}
	for _, field := range fields {
		wanted[field] = true
	}
	for d.More() {
		token, err := d.Token()
		field, ok := token.(string)
		if err != nil || !ok || !wanted[field] || seen[field] {
			return ErrInput
		}
		seen[field] = true
		var body json.RawMessage
		if d.Decode(&body) != nil || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
			return ErrInput
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || len(seen) != len(wanted) {
		return ErrInput
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInput
	}
	if json.Unmarshal(raw, value) != nil {
		return ErrInput
	}
	return nil
}

// UniqueOptions rejects repeated options except those explicitly repeated by the command.
func UniqueOptions(args []string, repeatable ...string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && !slices.Contains(repeatable, name) {
			return false
		}
		seen[name] = true
	}
	return true
}
