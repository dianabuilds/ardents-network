package releasedecision

import (
	"encoding/json"
	"os"
)

// decodeMetadataJSON parses the supplied TUF metadata envelope into a
// generic map. Tests use it to manipulate signatures, meta entries,
// and unknown fields without going through the typed metadata
// structure.
func decodeMetadataJSON(data []byte) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// jsonMarshalNoEscape encodes the supplied value as canonical JSON
// without HTML escaping. The TUF client expects unescaped JSON.
func jsonMarshalNoEscape(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// osWriteFile writes data to path, creating or truncating the file.
func osWriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
