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
	var actual, expected any
	if Decode(value.Data, &actual) != nil || Decode(encoded, &expected) != nil {
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
