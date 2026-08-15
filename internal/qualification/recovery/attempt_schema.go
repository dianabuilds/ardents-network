package recovery

import "encoding/json"

func attemptSchema(raw json.RawMessage) string {
	var value struct{ Schema string }
	if len(raw) == 0 || len(raw) > 2<<20 || json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value.Schema
}
