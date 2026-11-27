package systemd

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/godbus/dbus/v5"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

func startupProperties(answer *dbus.Call) (observation.Properties, error) {
	// Store's map conversion unwraps and recreates variants from reflected
	// values, losing original struct signatures (for example ExecStartEx).
	// Consume the already verified decoded dictionary without conversion.
	if len(answer.Body) != 1 {
		return nil, ErrObservation
	}
	values, ok := answer.Body[0].(map[string]dbus.Variant)
	if !ok || values == nil {
		return nil, ErrObservation
	}
	properties := make(observation.Properties, len(values))
	encodedSize := 2 // braces of the existing bounded property dictionary
	for key, value := range values {
		body, err := managerJSONValue(value.Value(), 0)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, errors.Join(ErrObservation, err)
		}
		signature := value.Signature().String()
		encodedKey, keyErr := managerJSONQuotedSize(key)
		encodedSignature, signatureErr := managerJSONQuotedSize(signature)
		if keyErr != nil || signatureErr != nil {
			return nil, ErrObservation
		}
		if len(properties) != 0 {
			encodedSize++
		}
		encodedSize += encodedKey + 1 + len(`{"type":`) + encodedSignature + len(`,"data":`) + len(raw) + 1
		if encodedSize > 64<<10 {
			return nil, ErrObservation
		}
		properties[key] = observation.Value{Type: signature, Data: raw}
	}
	// Preserve the existing bounded typed JSON vocabulary, including exact
	// integer values and byte arrays rather than JSON's base64 []byte form.
	// Input signatures/lengths/duplicates were already checked on the actual
	// frame. Re-encoding and decoding our own complete map adds no observation.
	return properties, nil
}

func managerJSONValue(value any, depth int) (any, error) {
	if depth > 16 {
		return nil, ErrObservation
	}
	switch typed := value.(type) {
	case []string:
		if len(typed) != 0 && depth == 16 {
			return nil, ErrObservation
		}
		if typed == nil {
			return []string{}, nil
		}
		return typed, nil
	case []uint32:
		if len(typed) != 0 && depth == 16 {
			return nil, ErrObservation
		}
		if typed == nil {
			return []uint32{}, nil
		}
		return typed, nil
	case []uint64:
		if len(typed) != 0 && depth == 16 {
			return nil, ErrObservation
		}
		if typed == nil {
			return []uint64{}, nil
		}
		return typed, nil
	case []byte:
		if len(typed) != 0 && depth == 16 {
			return nil, ErrObservation
		}
		// Preserve the original numeric JSON array, never encoding/json's
		// base64 byte-slice convention. Exact native depth/size rules remain.
		numbers := make([]uint16, len(typed))
		for i, value := range typed {
			numbers[i] = uint16(value)
		}
		return numbers, nil
	case dbus.Signature:
		return typed.String(), nil
	case dbus.Variant:
		body, err := managerJSONValue(typed.Value(), depth+1)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(body)
		return observation.Value{Type: typed.Signature().String(), Data: raw}, err
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return nil, ErrObservation
	}
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		result := make([]any, reflected.Len())
		for index := range result {
			child, err := managerJSONValue(reflected.Index(index).Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = child
		}
		return result, nil
	case reflect.Map:
		// busctl dictionaries preserve their ordered key/value entry shape.
		entries := make([]any, 0, reflected.Len())
		iterator := reflected.MapRange()
		for iterator.Next() {
			key, err := managerJSONValue(iterator.Key().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			child, err := managerJSONValue(iterator.Value().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			entries = append(entries, []any{key, child})
		}
		return entries, nil
	default:
		return value, nil
	}
}

// Only the length is needed for the original complete typed-dictionary bound.
// Safe ASCII needs no JSON allocation; every escaping/UTF-8 case retains the
// standard encoder. This caches neither input bytes nor an accepting verdict.
func managerJSONQuotedSize(value string) (int, error) {
	for i := range len(value) {
		c := value[i]
		if c < 0x20 || c > 0x7e || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			raw, err := json.Marshal(value)
			return len(raw), err
		}
	}
	return len(value) + 2, nil
}
