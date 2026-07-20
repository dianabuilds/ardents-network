package event

import "strings"

func CloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func Map(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSensitiveKey(k) {
			out[k] = "[redacted]"
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return Map(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = redactValue(typed[i])
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.Contains(key, "secret"):
		return true
	case strings.Contains(key, "password"):
		return true
	case strings.Contains(key, "token"):
		return true
	case strings.Contains(key, "plaintext"):
		return true
	case strings.Contains(key, "payload"):
		return true
	case strings.Contains(key, "ciphertext"):
		return true
	case strings.Contains(key, "nonce"):
		return true
	case strings.Contains(key, "key_material"):
		return true
	case strings.Contains(key, "private_key"):
		return true
	case strings.Contains(key, "seed"):
		return true
	case key == "key":
		return true
	default:
		return false
	}
}
