package releasedecision

import (
	"encoding/json"
	"errors"
	"fmt"
)

// metadataEnvelope is the parsed view of one metadata envelope, used
// for the reject-only resource preflight. The package never validates
// or normalizes TUF data here: that is go-tuf's responsibility.
type metadataEnvelope struct {
	role        string
	keys        map[string]struct{}
	roles       map[string]struct{}
	delegations bool
}

// validateInputsEnvelope is the reject-only resource preflight. It
// refuses inputs that exceed the published profile before contacting
// go-tuf, and it never turns rejected input into accepted input.
func validateInputsEnvelope(in Inputs) error {
	if int64(len(in.RootBytes)) > maximumMetadataFileBytes {
		return errors.New("trusted root exceeds the per-file bound")
	}
	if in.TargetPath == "" {
		return errors.New("target path is missing")
	}
	if len(in.Artifact) == 0 {
		return errors.New("artifact bytes are missing")
	}
	if in.Files == nil {
		return errors.New("metadata files are missing")
	}
	aggregate := int64(len(in.RootBytes))
	fetches := 0
	keys := make(map[string]struct{})
	roles := make(map[string]struct{})
	signatures := make(map[string]struct{})
	hasDelegations := false
	for path, data := range in.Files {
		fetches++
		if fetches > maximumFetches {
			return errors.New("file count exceeds the fetch bound")
		}
		if int64(len(data)) > maximumMetadataFileBytes {
			return fmt.Errorf("file %q exceeds the per-file bound", path)
		}
		aggregate += int64(len(data))
		if aggregate > maximumMetadataBytes {
			return errors.New("aggregate metadata exceeds the bound")
		}
		envelope, err := parseMetadataEnvelope(data)
		if err != nil {
			return fmt.Errorf("file %q is not a recognized metadata envelope: %w", path, err)
		}
		for key := range envelope.keys {
			keys[key] = struct{}{}
		}
		for role := range envelope.roles {
			roles[role] = struct{}{}
		}
		signatures[envelope.role] = struct{}{}
		if envelope.delegations {
			hasDelegations = true
		}
	}
	if len(keys) > maximumKeys {
		return errors.New("key count exceeds the bound")
	}
	if len(roles) > maximumRoles {
		return errors.New("role count exceeds the bound")
	}
	if len(signatures) > maximumSignatures {
		return errors.New("signature role count exceeds the bound")
	}
	if hasDelegations {
		return errors.New("delegated targets are disabled")
	}
	return nil
}

// parseMetadataEnvelope walks the JSON envelope to extract the bounded
// preflight facts. It performs no verification; it only bounds the
// resource envelope.
func parseMetadataEnvelope(data []byte) (metadataEnvelope, error) {
	var envelope struct {
		Signed     json.RawMessage   `json:"signed"`
		Signatures []json.RawMessage `json:"signatures"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return metadataEnvelope{}, err
	}
	if len(envelope.Signatures) > maximumSignatures {
		return metadataEnvelope{}, errors.New("signature count exceeds the bound")
	}
	var signed struct {
		Type        string                     `json:"_type"`
		Keys        map[string]json.RawMessage `json:"keys"`
		Roles       map[string]json.RawMessage `json:"roles"`
		Meta        map[string]json.RawMessage `json:"meta"`
		Targets     map[string]json.RawMessage `json:"targets"`
		Delegations json.RawMessage            `json:"delegations"`
	}
	if err := json.Unmarshal(envelope.Signed, &signed); err != nil {
		return metadataEnvelope{}, err
	}
	keys := make(map[string]struct{}, len(signed.Keys))
	for key := range signed.Keys {
		keys[key] = struct{}{}
	}
	roles := make(map[string]struct{}, len(signed.Roles))
	for role := range signed.Roles {
		roles[role] = struct{}{}
	}
	if signed.Type == targetRole && len(signed.Delegations) > 0 && string(signed.Delegations) != "null" {
		return metadataEnvelope{role: signed.Type, keys: keys, roles: roles, delegations: true}, nil
	}
	return metadataEnvelope{role: signed.Type, keys: keys, roles: roles}, nil
}
