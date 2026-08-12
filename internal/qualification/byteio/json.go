package byteio

import (
	"encoding/json"
	"errors"
	"os"
)

// WriteJSON writes one indented newline-terminated JSON file within maximum bytes.
func WriteJSON(path string, value any, maximum int) error {
	if maximum < 1 {
		return errors.New("json byte bound is invalid")
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > maximum {
		return errors.New("json exceeds its byte bound")
	}
	return os.WriteFile(path, raw, 0o600)
}
