package artifacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"time"
)

// DecodeObject preserves exact field spelling and fixed-array sizes in a typed
// local declaration. The caller owns required fields and semantic validation.
func DecodeObject(raw []byte, value any) error {
	target := reflect.TypeOf(value)
	if target == nil || target.Kind() != reflect.Pointer {
		return ErrInput
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := UniqueJSON(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing input")
	}
	if err := exactObjectFields(raw, target.Elem()); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(value)
}

// encoding/json deliberately accepts case-insensitive field aliases. Local
// authority/configuration inputs require the exact documented field spelling.
func exactObjectFields(raw []byte, t reflect.Type) error {
	if t == reflect.TypeFor[time.Time]() {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		allowed := map[string]reflect.Type{}
		for f := range t.Fields() {
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			allowed[name] = f.Type
		}
		for name, value := range fields {
			kind, ok := allowed[name]
			if !ok {
				return errors.New("unknown field")
			}
			if err := exactObjectFields(value, kind); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		// A named type with its own JSON decoder owns its wire grammar. In
		// particular, public Network preparation IDs require exact hex strings
		// rather than encoding/json's default integer-array representation.
		decoder := reflect.TypeFor[json.Unmarshaler]()
		if t.Implements(decoder) || reflect.PointerTo(t).Implements(decoder) {
			return nil
		}
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return err
		}
		// encoding/json pads short arrays and discards excess elements. Refuse
		// either change before decoding identities or fixed command bounds.
		if t.Kind() == reflect.Array && len(values) != t.Len() {
			return errors.New("invalid array length")
		}
		for _, v := range values {
			if err := exactObjectFields(v, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		return exactObjectFields(raw, t.Elem())
	}
	return nil
}

// UniqueJSON checks the next JSON value for duplicate keys and nulls.
// The caller owns trailing-input and schema checks.
func UniqueJSON(d *json.Decoder) error {
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
			if err := UniqueJSON(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	case json.Delim('['):
		for d.More() {
			if err := UniqueJSON(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}
