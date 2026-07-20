package config

import (
	"encoding/json"
)

func redactDocument(doc Document) map[string]any {
	raw, _ := json.Marshal(doc)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	redactMapValue(out, "api", "token_file")
	redactMapValue(out, "observability", "token_file")
	redactMapValue(out, "network", "private_key_path")
	redactNestedMapValue(out, "network", "wss", "private_key_file")
	for _, field := range []string{"capability_store", "capability_store_key_file", "replay_key_file", "subject"} {
		redactMapValue(out, "privacy", field)
	}
	redactMapEntries(out, "privacy", "trusted_issuers")
	redactNestedMapValue(out, "privacy", "discovery", "reference")
	redactNestedMapValue(out, "privacy", "discovery", "replay_path")
	redactNestedMapValue(out, "privacy", "data", "reference")
	redactNestedMapValue(out, "privacy", "data", "replay_path")
	return out
}

func redactMapEntries(root map[string]any, section, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	entries, ok := value[field].(map[string]any)
	if !ok {
		return
	}
	for key := range entries {
		entries[key] = "configured"
	}
}

func redactMapValue(root map[string]any, section, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	value[field] = configuredState(value[field])
}

func redactNestedMapValue(root map[string]any, section, nested, field string) {
	value, ok := root[section].(map[string]any)
	if !ok {
		return
	}
	child, ok := value[nested].(map[string]any)
	if !ok {
		return
	}
	child[field] = configuredState(child[field])
}

func configuredState(value any) string {
	text, _ := value.(string)
	if text == "" {
		return "missing"
	}
	return "configured"
}
