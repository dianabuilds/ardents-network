package targetlink

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// Read reads one bounded line and preserves the exact destination.
func Read(input io.Reader) (string, error) {
	if input == nil {
		return "", endpointstream.ErrInvalidLocalInput
	}
	line, err := bufio.NewReaderSize(input, 514).ReadSlice('\n')
	if err != nil && err != io.EOF {
		return "", endpointstream.ErrInvalidLocalInput
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	request := endpointstream.Request{Destination: endpointstream.TargetLink, Value: string(line)}
	if err := endpointstream.ValidateRequest(request); err != nil {
		return "", endpointstream.ErrInvalidLocalInput
	}
	return request.Value, nil
}

// ReadPrivate reads one Target Link from an owner-private regular
// file. It closes the original handle before returning and withholds local
// filesystem details from the caller.
func ReadPrivate(path string) (string, error) {
	unavailable := errors.New("private Target Link file is unavailable")
	input, err := openPrivateFile(path)
	if err != nil {
		return "", unavailable
	}
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		if closeErr := input.Close(); closeErr != nil {
			return "", errors.Join(unavailable, errors.New("private Target Link close failed"))
		}
		return "", unavailable
	}
	link, err := Read(input)
	if errors.Join(err, input.Close()) != nil {
		return "", unavailable
	}
	return link, nil
}
