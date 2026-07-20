package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func rejectDeprecatedFields(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	if _, found := root["version"]; found {
		return fmt.Errorf("deprecated field version: use api_version")
	}
	var network map[string]json.RawMessage
	if value, found := root["network"]; found && json.Unmarshal(value, &network) == nil {
		if _, deprecated := network["transport_mode"]; deprecated {
			return fmt.Errorf("deprecated field network.transport_mode: use network.transport_profile")
		}
	}
	return nil
}

func rejectDuplicateFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return scanJSONValue(decoder, "$")
}

func scanJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delim {
	case '{':
		return scanJSONObject(decoder, path)
	case '[':
		return scanJSONArray(decoder, path)
	default:
		return fmt.Errorf("invalid JSON delimiter at %s", path)
	}
}

func scanJSONObject(decoder *json.Decoder, path string) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("invalid JSON object key at %s", path)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("duplicate field %s.%s", path, name)
		}
		seen[name] = struct{}{}
		if err := scanJSONValue(decoder, path+"."+name); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func scanJSONArray(decoder *json.Decoder, path string) error {
	for index := 0; decoder.More(); index++ {
		if err := scanJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}
