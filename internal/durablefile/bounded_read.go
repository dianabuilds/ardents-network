package durablefile

import (
	"errors"
	"io"
	"os"
)

// ReadBounded reads one whole file or fails when it exceeds its bound. The
// caller supplies its exact bound-refusal prose; the mechanism reads at most
// maximum+1 bytes so an oversized file is refused without unbounded allocation.
func ReadBounded(path string, maximum int64, boundRefusal string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
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
