package durablefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadWatermark reads the canonical "<generation> <name>\n" floor file through
// the caller's bounded reader (which keeps its own bound-refusal prose). It
// returns exists=false only when the file is absent; any malformed floor is
// refused with the caller's invalid-watermark prose. The generation must be
// nonzero and the name a canonical generation digest, re-serialized exactly.
func LoadWatermark(root string, readBounded func(string, int64) ([]byte, error), invalidRefusal string) (uint64, string, bool, error) {
	raw, err := readBounded(filepath.Join(root, "watermark"), 96)
	if os.IsNotExist(err) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	line := strings.TrimSuffix(string(raw), "\n")
	parts := strings.Split(line, " ")
	if len(parts) != 2 || string(raw) != line+"\n" || !GenerationName.MatchString(parts[1]) {
		return 0, "", false, errors.New(invalidRefusal)
	}
	generation, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || generation == 0 || fmt.Sprintf("%d %s\n", generation, parts[1]) != string(raw) {
		return 0, "", false, errors.New(invalidRefusal)
	}
	return generation, parts[1], true, nil
}

// ReplaceWatermark durably writes the canonical floor for one nonzero generation
// and canonical name, refusing an invalid value with the caller's prose.
func ReplaceWatermark(root string, generation uint64, name string, invalidRefusal string) error {
	if generation == 0 || !GenerationName.MatchString(name) {
		return errors.New(invalidRefusal)
	}
	return ReplaceFile(root, ".watermark-", "watermark", []byte(fmt.Sprintf("%d %s\n", generation, name)))
}
