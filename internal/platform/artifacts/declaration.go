package artifacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// ErrInput reports an unavailable or invalid original artifact input.
var ErrInput = errors.New("original source preparation input unavailable")

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
