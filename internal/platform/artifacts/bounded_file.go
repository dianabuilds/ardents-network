package artifacts

import (
	"errors"
	"io"
	"math"
)

// ReadBoundedFile reads one regular file within the given byte bound.
func ReadBoundedFile(filename string, maximum int64) ([]byte, error) {
	// Opening must itself be nonblocking for a substituted FIFO. A path Stat
	// cannot protect the later Open. Only the opened object's type is used;
	// this remains bounded input reading, not filesystem custody.
	f, err := openInputFile(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("input unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(f, boundedReadLimit(maximum)))
	if err != nil || int64(len(raw)) > maximum {
		clear(raw)
		return nil, errors.New("input unavailable")
	}
	return raw, nil
}

// Read one extra byte to detect oversized input when the limit can represent it.
func boundedReadLimit(maximum int64) int64 {
	if maximum == math.MaxInt64 {
		return maximum
	}
	return maximum + 1
}
