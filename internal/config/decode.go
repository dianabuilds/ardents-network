// Package config owns operator document decoding, defaults, validation, precedence, and change classification.
// It does not own runtime composition or adapter-specific startup.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const MaxDocumentBytes = 1 << 20

func Decode(r io.Reader) (Document, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("read operator configuration: %w", err)
	}
	if len(raw) > MaxDocumentBytes {
		return Document{}, fmt.Errorf("operator configuration exceeds %d byte limit", MaxDocumentBytes)
	}
	if err := rejectDuplicateFields(raw); err != nil {
		return Document{}, err
	}
	doc := Defaults()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode operator configuration: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Document{}, err
	}
	applyContextDefaults(raw, &doc)
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode operator configuration: %w", err)
	}
	return fmt.Errorf("decode operator configuration: multiple JSON values")
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

const OperatorFileEnv = "ARDENTS_CONFIG_FILE"

func OperatorFile() string { return strings.TrimSpace(os.Getenv(OperatorFileEnv)) }

var obsoleteCredentialEnvironment = [...]string{
	"ARDENTS_API_TOKEN",
	"ARDENTS_API_TOKEN_FILE",
	"ARDENTS_APPLICATION_TOKEN",
	"ARDENTS_APPLICATION_TOKEN_FILE",
	"ARDENTS_LEGACY_API_TOKEN",
	"ARDENTS_LEGACY_TOKEN_FILE",
	"ARDENTS_TOKEN",
	"ARDENTS_TOKEN_FILE",
}

// RejectObsoleteCredentialEnvironment prevents a stale deployment environment
// from appearing to configure authentication. Values are never included in the
// error because they may contain retired bearer secrets.
func RejectObsoleteCredentialEnvironment() error {
	for _, name := range obsoleteCredentialEnvironment {
		if _, present := os.LookupEnv(name); present {
			return fmt.Errorf("obsolete credential environment variable %s is not supported", name)
		}
	}
	return nil
}
