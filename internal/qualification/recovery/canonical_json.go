package recovery

import (
	"bytes"
	"encoding/json"
)

func canonicalJSONEqual(raw, canonical []byte) bool {
	return bytes.Equal(compactJSON(raw), canonical)
}

func compactJSON(raw []byte) []byte {
	var compact bytes.Buffer
	if json.Compact(&compact, raw) != nil {
		return raw
	}
	return compact.Bytes()
}

func jsonDigest(raw []byte) string { return hexDigest(compactJSON(raw)) }
