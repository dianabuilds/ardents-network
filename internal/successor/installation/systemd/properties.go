package systemd

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"unicode/utf8"
)

type Value struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Properties map[string]Value

// Preserve signatures, integer precision and absence. Duplicate fields cannot
// replace an earlier required observation, even inside a variant dictionary.
func Decode(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > 64<<10 || !utf8.Valid(raw) {
		return ErrObservation
	}
	tokens := json.NewDecoder(bytes.NewReader(raw))
	tokens.UseNumber()
	if err := observeJSONValue(tokens, 0); err != nil {
		return err
	}
	if _, err := tokens.Token(); err != io.EOF {
		return ErrObservation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return ErrObservation
	}
	return nil
}

func observeJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return ErrObservation
	}
	token, err := decoder.Token()
	if err != nil {
		return ErrObservation
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return ErrObservation
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			name, stringKey := key.(string)
			if err != nil || !stringKey || seen[name] {
				return ErrObservation
			}
			seen[name] = true
		}
		if err := observeJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	closeToken, err := decoder.Token()
	wanted := json.Delim(']')
	if delimiter == '{' {
		wanted = '}'
	}
	if err != nil || closeToken != wanted {
		return ErrObservation
	}
	return nil
}

func Matches(properties Properties, name, signature string, wanted any) bool {
	value, found := properties[name]
	if !found || value.Type != signature {
		return false
	}
	encoded, err := json.Marshal(wanted)
	if err != nil {
		return false
	}
	if len(encoded) > 64<<10 {
		return false
	}
	if bytes.Equal(value.Data, encoded) {
		// These built-in values cannot supply MarshalJSON, duplicate object
		// keys or excessive nesting. Marshal has already validated the finite
		// number/string encoding; equality carries that proof to fresh bytes.
		switch wanted.(type) {
		case bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, []string:
			return true
		}
	}
	var actual, expected any
	if Decode(encoded, &expected) != nil {
		return false
	}
	// The expected encoding has passed the same size, depth and JSON checks.
	// Byte equality proves those checks for this fresh observed value too;
	// parsing identical bytes again adds no observation or type evidence.
	if bytes.Equal(value.Data, encoded) {
		return true
	}
	if Decode(value.Data, &actual) != nil {
		return false
	}
	return reflect.DeepEqual(actual, expected)
}

func Contains(properties Properties, name string, required []string) bool {
	value, present := properties[name]
	var items []string
	if !present || value.Type != "as" || Decode(value.Data, &items) != nil || items == nil {
		return false
	}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item] {
			return false
		}
		seen[item] = true
	}
	for _, item := range required {
		if !seen[item] {
			return false
		}
	}
	return true
}
