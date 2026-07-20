package event

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapRedactsSensitiveFields(t *testing.T) {
	payload := Map(map[string]any{
		"secret":       "top-secret",
		"safe_field":   "ok",
		"key_material": "123",
		"nested":       map[string]any{"token": "abc"},
	})

	require.Falsef(t, payload["secret"] != "[redacted]", "secret field = %#v, want redacted", payload["secret"])
	require.Falsef(t, payload["key_material"] != "[redacted]", "key_material field = %#v, want redacted", payload["key_material"])
	require.Falsef(t, payload["safe_field"] != "ok", "safe field = %#v, want ok", payload["safe_field"])

	nested, ok := payload["nested"].(map[string]any)
	require.Truef(t, ok, "nested payload type = %T, want map[string]any", payload["nested"])
	require.Falsef(t, nested["token"] != "[redacted]", "nested token = %#v, want redacted", nested["token"])
}

func TestMapRedactsSensitiveValuesInsideArrays(t *testing.T) {
	payload := Map(map[string]any{
		"items": []any{
			map[string]any{"token": "abc"},
			map[string]any{"safe": "ok", "nested": []any{map[string]any{"plaintext": "secret"}}},
		},
	})

	items, ok := payload["items"].([]any)
	require.Falsef(t, !ok || len(items) != 2, "items payload = %#v, want two array entries", payload["items"])

	first, ok := items[0].(map[string]any)
	require.Falsef(t, !ok || first["token"] != "[redacted]", "first array item = %#v, want token redacted", items[0])

	second, ok := items[1].(map[string]any)
	require.Falsef(t, !ok || second["safe"] != "ok", "second array item = %#v, want safe field preserved", items[1])

	nested, ok := second["nested"].([]any)
	require.Falsef(t, !ok || len(nested) != 1, "nested array = %#v, want one entry", second["nested"])

	inner, ok := nested[0].(map[string]any)
	require.Falsef(t, !ok || inner["plaintext"] != "[redacted]", "nested array item = %#v, want plaintext redacted", nested[0])
}
