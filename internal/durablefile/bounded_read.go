package durablefile

import (
	"errors"
	"io"
	"math"
	"os"
)

// ReadBounded reads one whole file or fails when it exceeds its bound. The
// caller supplies its exact bound-refusal prose; the mechanism reads at most
// one byte past the bound when that read limit is representable.
func ReadBounded(path string, maximum int64, boundRefusal string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	readLimit := maximum
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, readLimit))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(raw)) > maximum {
		return nil, errors.New(boundRefusal)
	}
	return raw, nil
}
