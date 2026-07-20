package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	if err := rejectDeprecatedFields(raw); err != nil {
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
