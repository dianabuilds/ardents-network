package endpoint

import (
	"bytes"
	"encoding/json"
)

func hasPermissionEvent(journal []byte, phase, role string) bool {
	for _, line := range bytes.Split(journal, []byte{'\n'}) {
		var event struct {
			Phase string `json:"phase"`
			Role  string `json:"role"`
		}
		if json.Unmarshal(line, &event) == nil && event.Phase == phase && event.Role == role {
			return true
		}
	}
	return false
}
