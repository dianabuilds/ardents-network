package connection

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
