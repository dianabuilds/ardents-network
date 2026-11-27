package endpointstream

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// ErrInvalidTargetLink identifies an invalid local Target Link input.
var ErrInvalidTargetLink = errors.New("document destination or local input is invalid")

// ReadTargetLink reads one bounded line and preserves the exact destination.
func ReadTargetLink(input io.Reader) (string, error) {
	if input == nil {
		return "", ErrInvalidTargetLink
	}
	line, err := bufio.NewReaderSize(input, 514).ReadSlice('\n')
	if err != nil && err != io.EOF {
		return "", ErrInvalidTargetLink
	}
	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	request := Request{Destination: TargetLink, Value: string(line)}
	if _, err := EncodeRequest(request); err != nil {
		return "", ErrInvalidTargetLink
	}
	return request.Value, nil
}

// ReadPrivateTargetLink reads one Target Link from an owner-private regular
// file. It closes the original handle before returning and withholds local
// filesystem details from the caller.
func ReadPrivateTargetLink(path string) (string, error) {
	unavailable := errors.New("private Target Link file is unavailable")
	input, err := openPrivateLinkFile(path)
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
	link, err := ReadTargetLink(input)
	if errors.Join(err, input.Close()) != nil {
		return "", unavailable
	}
	return link, nil
}
