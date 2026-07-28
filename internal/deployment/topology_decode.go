package deployment

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ValidationError is one stable redacted topology rejection code.
type ValidationError string

func (err ValidationError) Error() string { return string(err) }

func decodeTopology(raw []byte) (topologyManifest, error) {
	if len(raw) > MaxTopologyBytes {
		return topologyManifest{}, ValidationError("topology_manifest_too_large")
	}
	if err := rejectDuplicateTopologyFields(raw); err != nil {
		return topologyManifest{}, err
	}
	var manifest topologyManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return topologyManifest{}, ValidationError("topology_unknown_field")
		}
		return topologyManifest{}, ValidationError("topology_invalid_json")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return topologyManifest{}, ValidationError("topology_trailing_value")
	}
	return manifest, nil
}

func rejectDuplicateTopologyFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanTopologyValue(decoder); err != nil {
		if errors.Is(err, errForbiddenTopologyField) {
			return ValidationError("topology_forbidden_secret_field")
		}
		if errors.Is(err, errDuplicateTopologyField) {
			return ValidationError("topology_duplicate_field")
		}
		if errors.Is(err, errNonCanonicalTopologyField) {
			return ValidationError("topology_unknown_field")
		}
		return ValidationError("topology_invalid_json")
	}
	return nil
}

var (
	errDuplicateTopologyField    = errors.New("duplicate topology field")
	errForbiddenTopologyField    = errors.New("forbidden topology field")
	errNonCanonicalTopologyField = errors.New("non-canonical topology field")
)

func scanTopologyValue(decoder *json.Decoder) error {
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
		seen := make(map[string]struct{})
		for decoder.More() {
			token, err = decoder.Token()
			if err != nil {
				return err
			}
			name, ok := token.(string)
			if !ok {
				return ValidationError("topology_invalid_json")
			}
			if forbiddenTopologyField(name) {
				return errForbiddenTopologyField
			}
			if name != strings.ToLower(name) {
				return errNonCanonicalTopologyField
			}
			if _, duplicate := seen[name]; duplicate {
				return errDuplicateTopologyField
			}
			seen[name] = struct{}{}
			if err := scanTopologyValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanTopologyValue(decoder); err != nil {
				return err
			}
		}
	default:
		return ValidationError("topology_invalid_json")
	}
	_, err = decoder.Token()
	return err
}

func forbiddenTopologyField(name string) bool {
	value := strings.ToLower(name)
	for _, forbidden := range []string{
		"password", "private_key", "signer_path", "signer_file", "session",
		"credential", "token", "api_key", "channel_grant", "channel_secret", "selector",
		"certificate_key", "repository_credentials",
	} {
		if strings.Contains(value, forbidden) {
			return true
		}
	}
	return false
}
