package endpoint

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// readPlanInput owns the bounded file boundary for Endpoint's one accepted
// operator plan. It is not a reusable plan abstraction.
func readPlanInput(path string, maximum int64) ([]byte, error) {
	if maximum <= 0 {
		return nil, errors.New("endpoint plan bound must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(contents)) > maximum {
		return nil, errors.New("endpoint plan exceeds its bound")
	}
	return contents, nil
}

func decodeEndpointPlan(path string, maximum int64, target any) error {
	raw, err := readPlanInput(path, maximum)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("endpoint plan contains trailing JSON")
	}
	return nil
}

func decodeEndpointFixedHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != len(destination) {
		return fmt.Errorf("invalid fixed hexadecimal value")
	}
	copy(destination, decoded)
	return nil
}
