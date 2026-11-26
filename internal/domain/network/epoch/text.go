package epoch

import (
	"errors"
	"unicode/utf8"

	"github.com/dianabuilds/ardents-network/internal/domain/network/canonicalcursor"
)

// decodeText reads one bounded canonical text field from the shared cursor:
// a nonzero length byte within the caller's maximum, UTF-8 bytes and the
// printing range 0x21..0x7e. The text grammar and its refusal prose belong to
// Epoch; the bounded read itself belongs to the shared canonical cursor.
func decodeText(d *canonicalcursor.Decoder, maximum int) (string, error) {
	rawLength, err := d.Bytes(1)
	if err != nil {
		return "", err
	}
	length := rawLength[0]
	if length == 0 || int(length) > maximum {
		return "", errors.New("canonical text length is invalid")
	}
	value, err := d.Bytes(int(length))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", errors.New("canonical text is not UTF-8")
	}
	for _, current := range value {
		if current < 0x21 || current > 0x7e {
			return "", errors.New("canonical text contains a non-printing byte")
		}
	}
	return string(value), nil
}
